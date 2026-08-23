package curl

import (
	"strings"

	"github.com/cristianradulescu/httpfly/internal/httpfile"
)

// ToCurl renders req as a multi-line, bash-style curl command -- the
// reverse of Parse. Method is always emitted explicitly via -X (even for
// GET), so the reader never has to wonder whether it was inferred.
// Headers are emitted as plain "-H 'Name: Value'" lines in their original
// order; a Cookie or Basic-auth Authorization header is not un-mapped
// back into -b/-u, since a plain header is equally valid curl and doesn't
// require guessing intent.
func ToCurl(req httpfile.Request) string {
	lines := []string{
		"curl " + shellQuote(req.URL),
		"-X " + shellQuote(req.Method),
	}
	for _, h := range req.Headers {
		lines = append(lines, "-H "+shellQuote(h.Name+": "+h.Value))
	}
	if req.Proxy != "" {
		lines = append(lines, "-x "+shellQuote(req.Proxy))
	}
	if req.Body != "" {
		lines = append(lines, "--data-raw "+shellQuote(req.Body))
	}

	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteString("  ")
		}
		b.WriteString(line)
		if i < len(lines)-1 {
			b.WriteString(" \\\n")
		} else {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// shellQuote wraps s in single quotes for safe use as one bash argument.
// An embedded single quote is escaped using the standard bash idiom:
// close the quote, emit a backslash-escaped literal single quote, then
// reopen the quote.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
