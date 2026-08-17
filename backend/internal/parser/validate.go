package parser

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/cristianradulescu/httpfly/internal/httpfile"
	"github.com/cristianradulescu/httpfly/internal/interpolate"
)

// Severity is how serious a validation Issue is. A Warning means the file is
// syntactically valid .http but relies on something httpfly doesn't act on
// yet (an unimplemented feature, a non-standard method, ...). An Error means
// the block can't be turned into a request at all.
type Severity int

const (
	SeverityWarning Severity = iota
	SeverityError
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "ERROR"
	case SeverityWarning:
		return "WARN"
	default:
		return "UNKNOWN"
	}
}

// Issue is a single validation finding for one element of a request block.
type Issue struct {
	Element  string // e.g. "metadata:name", "method", "url", "proto", "header"
	Severity Severity
	Message  string
}

// BlockResult is the best-effort parsed request and validation issues for
// one ###-delimited block.
type BlockResult struct {
	Index   int
	Request httpfile.Request
	Issues  []Issue
}

// HasErrors reports whether any issue in the block is an error.
func (b BlockResult) HasErrors() bool {
	for _, issue := range b.Issues {
		if issue.Severity == SeverityError {
			return true
		}
	}
	return false
}

// Result is the outcome of analyzing an .http file: one BlockResult per
// request block, in file order, plus the file-scoped variables collected
// from every "@name = value" declaration.
type Result struct {
	Variables map[string]string
	Blocks    []BlockResult
}

// HasErrors reports whether any block has an error-level issue.
func (r Result) HasErrors() bool {
	for _, b := range r.Blocks {
		if b.HasErrors() {
			return true
		}
	}
	return false
}

// standardMethods are the HTTP methods httpfly recognizes without a warning.
var standardMethods = map[string]bool{
	"GET": true, "HEAD": true, "POST": true, "PUT": true, "DELETE": true,
	"CONNECT": true, "OPTIONS": true, "TRACE": true, "PATCH": true,
}

var protoPattern = regexp.MustCompile(`^HTTP/\d(\.\d)?$`)

// validateBlock parses and validates one ###-delimited block, resolving any
// "{{var}}" placeholders in the URL, headers, and body against vars. ok is
// false for blocks that contain only comments/blank/variable-declaration
// lines (e.g. the file-level prelude, or a trailing separator with nothing
// after it) -- those aren't requests and produce no issues.
func validateBlock(lines []string, vars map[string]string) (req httpfile.Request, issues []Issue, ok bool) {
	i := 0
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}
		if strings.HasPrefix(line, "#") {
			if key, value, isMetadata := parseMetadata(line); isMetadata {
				issues = append(issues, validateMetadata(key, value)...)
				if key == "name" {
					req.Name = value
				}
			}
			i++
			continue
		}
		if _, _, isVar := parseVariableDef(line); isVar {
			i++
			continue
		}
		break
	}

	if i >= len(lines) {
		return httpfile.Request{}, nil, false
	}

	line := strings.TrimSpace(lines[i])
	if strings.HasPrefix(line, "<") || strings.HasPrefix(line, ">") {
		issues = append(issues, Issue{
			Element:  "request-line",
			Severity: SeverityError,
			Message:  "pre/post-request scripting is not yet supported",
		})
		return req, issues, true
	}

	method, rawURL, proto, lineIssues := validateRequestLine(line, vars)
	req.Method, req.URL, req.Proto = method, rawURL, proto
	issues = append(issues, lineIssues...)
	i++

	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		i++
		if line == "" {
			break
		}
		header, headerIssues := validateHeader(line, vars)
		if len(headerIssues) == 0 {
			req.Headers = append(req.Headers, header)
		}
		issues = append(issues, headerIssues...)
	}

	body, missing := interpolate.Apply(trimBody(lines[i:]), vars)
	req.Body = body
	for _, name := range missing {
		issues = append(issues, undefinedVariableIssue("body", name))
	}

	return req, issues, true
}

func validateMetadata(key, value string) []Issue {
	if key == "" {
		return []Issue{{Element: "metadata", Severity: SeverityError, Message: "empty metadata key"}}
	}
	switch key {
	case "name":
		if value == "" {
			return []Issue{{Element: "metadata:name", Severity: SeverityError, Message: "@name requires a value"}}
		}
		return nil
	case "lang":
		return []Issue{{
			Element:  "metadata:lang",
			Severity: SeverityWarning,
			Message:  "recognized but not yet supported (pre/post-request scripting is unimplemented)",
		}}
	default:
		return []Issue{{
			Element:  "metadata:" + key,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("unknown metadata \"@%s\"", key),
		}}
	}
}

func validateRequestLine(line string, vars map[string]string) (method, resolvedURL, proto string, issues []Issue) {
	fields := strings.Fields(line)
	var rawURL string
	switch len(fields) {
	case 2:
		method, rawURL, proto = fields[0], fields[1], "HTTP/1.1"
	case 3:
		method, rawURL, proto = fields[0], fields[1], fields[2]
	default:
		return "", "", "", []Issue{{
			Element:  "request-line",
			Severity: SeverityError,
			Message:  fmt.Sprintf("expected \"METHOD URL [PROTO]\", got %q", line),
		}}
	}

	if !standardMethods[strings.ToUpper(method)] {
		issues = append(issues, Issue{
			Element:  "method",
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("non-standard HTTP method %q", method),
		})
	}

	resolvedURL, missing := interpolate.Apply(rawURL, vars)
	for _, name := range missing {
		issues = append(issues, undefinedVariableIssue("url", name))
	}

	if u, err := url.Parse(resolvedURL); err != nil {
		issues = append(issues, Issue{
			Element:  "url",
			Severity: SeverityError,
			Message:  fmt.Sprintf("invalid URL: %v", err),
		})
	} else if (u.Scheme == "" || u.Host == "") && len(missing) == 0 {
		// A still-unresolved placeholder already explains why this doesn't
		// look absolute; don't pile on a second issue for it.
		issues = append(issues, Issue{
			Element:  "url",
			Severity: SeverityError,
			Message:  "URL must be absolute (missing scheme or host)",
		})
	}

	if len(fields) == 3 && !protoPattern.MatchString(proto) {
		issues = append(issues, Issue{
			Element:  "proto",
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("unrecognized protocol version %q", proto),
		})
	}

	return method, resolvedURL, proto, issues
}

func validateHeader(line string, vars map[string]string) (httpfile.Header, []Issue) {
	name, rawValue, found := strings.Cut(line, ":")
	if !found {
		return httpfile.Header{}, []Issue{{
			Element:  "header",
			Severity: SeverityError,
			Message:  fmt.Sprintf("invalid header line %q, expected \"Name: Value\"", line),
		}}
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return httpfile.Header{}, []Issue{{Element: "header", Severity: SeverityError, Message: "empty header name"}}
	}

	value, missing := interpolate.Apply(strings.TrimSpace(rawValue), vars)
	var issues []Issue
	for _, varName := range missing {
		issues = append(issues, undefinedVariableIssue("header:"+name, varName))
	}
	return httpfile.Header{Name: name, Value: value}, issues
}

func undefinedVariableIssue(element, name string) Issue {
	return Issue{
		Element:  element,
		Severity: SeverityWarning,
		Message:  fmt.Sprintf("undefined variable %q", name),
	}
}
