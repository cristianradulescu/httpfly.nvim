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
	return hasError(b.Issues)
}

// Result is the outcome of analyzing an .http file: the global variables and
// metadata declared in the prelude (before the first "###"), any issues
// found there, and one BlockResult per request block, in file order.
type Result struct {
	Variables    map[string]string
	GlobalIssues []Issue
	Blocks       []BlockResult
}

// HasErrors reports whether the prelude or any block has an error-level
// issue.
func (r Result) HasErrors() bool {
	if hasError(r.GlobalIssues) {
		return true
	}
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

// metadataScope is where a "# @key value" line was found.
type metadataScope int

const (
	scopeGlobal metadataScope = iota
	scopeLocal
)

func (s metadataScope) String() string {
	if s == scopeGlobal {
		return "global"
	}
	return "per-request"
}

// localOnlyMetadata are known keys that only make sense attached to one
// specific request, never as a file-wide default.
var localOnlyMetadata = map[string]bool{
	"name": true, // the per-request identifier -- there's no sensible "global name"
	"lang": true, // names the language of *this* block's inline script
}

// knownMetadataKeys are the keys validateMetadata gives special handling to.
// Anything else is "unknown metadata" regardless of scope, so scope isn't
// enforced for it beyond that existing warning.
var knownMetadataKeys = map[string]bool{"name": true, "lang": true, "proxy": true}

// parsePrelude parses the file segment before the first "###": "key = value"
// lines become global variables, "# @key value" lines become global
// metadata (subject to the same scope rules as per-request metadata), and
// anything else is very likely a mistake (e.g. a request missing its
// leading "###") so it's reported rather than silently dropped.
func parsePrelude(lines []string) (vars map[string]string, metadata map[string]string, issues []Issue) {
	vars = make(map[string]string)
	metadata = make(map[string]string)

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		switch {
		case line == "":
			continue
		case strings.HasPrefix(line, "#"):
			key, value, isMetadata := parseMetadata(line)
			if !isMetadata {
				continue
			}
			scopeIssues := validateMetadataInScope(key, value, scopeGlobal)
			issues = append(issues, scopeIssues...)
			if !hasError(scopeIssues) {
				metadata[key] = value
			}
		default:
			if name, value, isVar := parseVariableDef(line); isVar {
				vars[name] = value
				continue
			}
			issues = append(issues, Issue{
				Element:  "global",
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("ignored: content before the first \"###\" separator: %q", line),
			})
		}
	}
	return vars, metadata, issues
}

// validateBlock parses and validates one ###-delimited block, merging its
// local variables/metadata over the file's global defaults (local wins) and
// resolving any "{{var}}" placeholders in the URL, headers, body, and proxy
// against the merged variables. ok is false for blocks that contain only
// comments/blank/variable-declaration lines -- those aren't requests and
// produce no issues.
func validateBlock(lines []string, globalVars, globalMetadata map[string]string) (req httpfile.Request, issues []Issue, ok bool) {
	i := 0
	nameDeclared := false
	localVars := make(map[string]string)
	localMetadata := make(map[string]string)

	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}
		if strings.HasPrefix(line, "#") {
			if key, value, isMetadata := parseMetadata(line); isMetadata {
				issues = append(issues, validateMetadataInScope(key, value, scopeLocal)...)
				if key == "name" {
					nameDeclared = true
					req.Name = value
				} else {
					localMetadata[key] = value
				}
			}
			i++
			continue
		}
		if name, value, isVar := parseVariableDef(line); isVar {
			localVars[name] = value
			i++
			continue
		}
		break
	}

	if i >= len(lines) {
		return httpfile.Request{}, nil, false
	}

	if !nameDeclared {
		issues = append(issues, Issue{
			Element:  "metadata:name",
			Severity: SeverityError,
			Message:  "missing required @name metadata",
		})
	}

	vars := mergeVars(globalVars, localVars)

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
		if !hasError(headerIssues) {
			req.Headers = append(req.Headers, header)
		}
		issues = append(issues, headerIssues...)
	}

	rawProxy := localMetadata["proxy"]
	if rawProxy == "" {
		rawProxy = globalMetadata["proxy"]
	}
	resolvedProxy, proxyIssues := resolveProxy(rawProxy, vars)
	req.Proxy = resolvedProxy
	issues = append(issues, proxyIssues...)

	body, missing := interpolate.Apply(trimBody(lines[i:]), vars)
	req.Body = body
	for _, name := range missing {
		issues = append(issues, undefinedVariableIssue("body", name))
	}

	return req, issues, true
}

func mergeVars(global, local map[string]string) map[string]string {
	merged := make(map[string]string, len(global)+len(local))
	for k, v := range global {
		merged[k] = v
	}
	for k, v := range local {
		merged[k] = v
	}
	return merged
}

// validateMetadataInScope runs the usual per-key checks and, for keys with
// a restricted scope (currently just the local-only ones), also flags the
// line if it showed up somewhere that key isn't allowed.
func validateMetadataInScope(key, value string, scope metadataScope) []Issue {
	issues := validateMetadata(key, value)
	if knownMetadataKeys[key] && scope == scopeGlobal && localOnlyMetadata[key] {
		issues = append(issues, Issue{
			Element:  "metadata:" + key,
			Severity: SeverityError,
			Message:  fmt.Sprintf("\"@%s\" is not allowed as global metadata (per-request only)", key),
		})
	}
	return issues
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
	case "proxy":
		if value == "" {
			return []Issue{{Element: "metadata:proxy", Severity: SeverityError, Message: "@proxy requires a value"}}
		}
		return nil
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

	resolvedURL, missing := interpolate.ApplyURL(rawURL, vars)
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

// resolveProxy interpolates and validates an effective "@proxy" value. An
// empty rawProxy (no proxy declared, locally or globally) is not an issue --
// it just means the request connects directly.
func resolveProxy(rawProxy string, vars map[string]string) (string, []Issue) {
	if rawProxy == "" {
		return "", nil
	}

	resolved, missing := interpolate.Apply(rawProxy, vars)
	var issues []Issue
	for _, name := range missing {
		issues = append(issues, undefinedVariableIssue("metadata:proxy", name))
	}

	if u, err := url.Parse(resolved); err != nil {
		issues = append(issues, Issue{
			Element:  "metadata:proxy",
			Severity: SeverityError,
			Message:  fmt.Sprintf("invalid proxy URL: %v", err),
		})
	} else if (u.Scheme == "" || u.Host == "") && len(missing) == 0 {
		issues = append(issues, Issue{
			Element:  "metadata:proxy",
			Severity: SeverityError,
			Message:  "proxy URL must be absolute (missing scheme or host)",
		})
	}

	return resolved, issues
}

func hasError(issues []Issue) bool {
	for _, issue := range issues {
		if issue.Severity == SeverityError {
			return true
		}
	}
	return false
}

func undefinedVariableIssue(element, name string) Issue {
	return Issue{
		Element:  element,
		Severity: SeverityWarning,
		Message:  fmt.Sprintf("undefined variable %q", name),
	}
}
