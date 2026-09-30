package curl

import (
	"fmt"
	"strings"

	"github.com/cristianradulescu/httpfly/internal/httpfile"
)

// Format renders req as a single "###"-delimited .http request block.
// req.Name should already be set by the caller (httpfly requires a
// non-empty @name on every request); Proto defaults to "HTTP/1.1" if
// unset.
func Format(req httpfile.Request) string {
	var b strings.Builder
	b.WriteString("###\n")
	fmt.Fprintf(&b, "# @name %s\n", req.Name)
	if req.Proxy != "" {
		fmt.Fprintf(&b, "# @proxy %s\n", req.Proxy)
	}

	proto := req.Proto
	if proto == "" {
		proto = "HTTP/1.1"
	}
	fmt.Fprintf(&b, "%s %s %s\n", req.Method, req.URL, proto)

	for _, h := range req.Headers {
		fmt.Fprintf(&b, "%s: %s\n", h.Name, h.Value)
	}

	if req.Body != "" {
		b.WriteString("\n")
		b.WriteString(req.Body)
		b.WriteString("\n")
	}

	return b.String()
}
