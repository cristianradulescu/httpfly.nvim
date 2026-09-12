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
//
// A body containing one of httpfly's own "< path/to/file" file references
// (see internal/parser's spliceFileReferences) is reconstructed from
// req.RawBody -- not req.Body -- specifically so the file is referenced by
// path in the generated command ("--data-binary @path", or "-F" per part
// for a multipart body) rather than the fully-resolved, byte-spliced
// content (req.Body) being dumped inline as a single huge/binary shell
// argument. RawBody is empty for a request built directly by Parse (which
// never splices anything -- see Parse's own "< path" output for -d @file),
// so the check falls back to Body in that case; RawBody is only actually
// preferred over Body when a resolved request (from the parser package)
// has already spliced a file in. One limitation this implies: a "{{var}}"
// inside the path itself is passed through literally rather than resolved
// (RawBody is pre-interpolation), since ToCurl -- unlike Resolve -- has no
// variable set to resolve it against; a literal, already-resolved path
// works fine.
func ToCurl(req httpfile.Request) string {
	lines := []string{
		"curl " + shellQuote(req.URL),
		"-X " + shellQuote(req.Method),
	}

	unsplicedBody := req.RawBody
	if unsplicedBody == "" {
		unsplicedBody = req.Body
	}
	formParts, isMultipart := parseMultipartBody(headerValue(req.Headers, "Content-Type"), unsplicedBody)

	for _, h := range req.Headers {
		if isMultipart && strings.EqualFold(h.Name, "Content-Type") {
			continue // a "-F" flag below sets its own multipart Content-Type
		}
		lines = append(lines, "-H "+shellQuote(h.Name+": "+h.Value))
	}
	if req.Proxy != "" {
		lines = append(lines, "-x "+shellQuote(req.Proxy))
	}

	switch {
	case isMultipart:
		for _, p := range formParts {
			lines = append(lines, "-F "+shellQuote(formatFormFlagValue(p)))
		}
	case wholeBodyIsFileReference(unsplicedBody):
		path := strings.TrimPrefix(strings.TrimSpace(unsplicedBody), "< ")
		lines = append(lines, "--data-binary "+shellQuote("@"+path))
	case req.Body != "":
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

// headerValue returns the value of the first header named name
// (case-insensitively), or "" if there isn't one.
func headerValue(headers []httpfile.Header, name string) string {
	for _, h := range headers {
		if strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
}

// wholeBodyIsFileReference reports whether rawBody, once trimmed, is
// exactly one "< path/to/file" line -- the whole-body (non-multipart)
// upload case.
func wholeBodyIsFileReference(rawBody string) bool {
	trimmed := strings.TrimSpace(rawBody)
	return strings.HasPrefix(trimmed, "< ") && !strings.Contains(trimmed, "\n")
}

// shellQuote wraps s in single quotes for safe use as one bash argument.
// An embedded single quote is escaped using the standard bash idiom:
// close the quote, emit a backslash-escaped literal single quote, then
// reopen the quote.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
