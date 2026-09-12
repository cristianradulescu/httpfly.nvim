package parser

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	lua "github.com/yuin/gopher-lua"

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

// undefinedVariablePrefix is how undefinedVariableIssue's Message always
// starts, so IsUndefinedVariableIssue can recognize one without a
// dedicated field on Issue.
const undefinedVariablePrefix = "undefined variable "

// IsUndefinedVariableIssue reports whether issue is the "undefined
// variable" warning Resolve emits for an unresolved "{{name}}" placeholder.
// A caller re-resolving right before actually using a request (e.g. the CLI
// immediately before sending) may want to treat this as fatal even though
// Analyze only ever reports it as a warning (at parse/validate time, a
// variable that's undefined now might still be set by a script before the
// request is actually sent).
func IsUndefinedVariableIssue(issue Issue) bool {
	return strings.HasPrefix(issue.Message, undefinedVariablePrefix)
}

// missingFilePrefix is how a spliceFileReferences read-failure Issue's
// Message always starts, so IsMissingFileIssue can recognize one without a
// dedicated field on Issue -- mirrors undefinedVariablePrefix above.
const missingFilePrefix = "could not read file "

// IsMissingFileIssue reports whether issue is the "could not read file"
// warning spliceFileReferences emits for a "< path/to/file" body reference
// that couldn't be read. Like IsUndefinedVariableIssue, a caller
// re-resolving right before actually sending a request may want to treat
// this as fatal even though Analyze only ever reports it as a warning --
// the file might not exist yet at validate/parse time but still get
// created by a pre-request script before the request is actually sent.
func IsMissingFileIssue(issue Issue) bool {
	return strings.HasPrefix(issue.Message, missingFilePrefix)
}

// fileReferencePattern matches a body line that references a file to
// splice in verbatim, JetBrains HTTP Client style: "< path/to/file".
var fileReferencePattern = regexp.MustCompile(`^<\s+(\S.*)$`)

// spliceFileReferences replaces every line in body that reads
// "< path/to/file" with that file's raw bytes, so a request can send a
// file's exact content -- a raw binary upload (the whole body is one such
// line), or one part of a multipart body -- without a pre-request script
// reading it in by hand. A relative path resolves against the process's
// current working directory, same as everything else httpfly reads from
// disk (os.ReadFile's own default; there's no separate ".http file's own
// directory" rule here -- see the config-dir precedent in CLAUDE.md).
//
// A file that can't be read is a warning, not a hard error: a pre-request
// script might still create it before the request is actually sent --
// see IsMissingFileIssue, which lets a caller escalate this the same way
// IsUndefinedVariableIssue already is.
func spliceFileReferences(body string) (string, []Issue) {
	if !strings.Contains(body, "<") {
		return body, nil
	}

	lines := strings.Split(body, "\n")
	var issues []Issue
	for i, line := range lines {
		m := fileReferencePattern.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		path := m[1]
		content, err := os.ReadFile(path)
		if err != nil {
			issues = append(issues, Issue{
				Element:  "body",
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("%s%q: %v", missingFilePrefix, path, err),
			})
			continue
		}
		lines[i] = string(content)
	}
	return strings.Join(lines, "\n"), issues
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

// parsePrelude parses the file segment before the first "###": "@key = value"
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

// validateBlock parses one ###-delimited block -- metadata, local variable
// declarations, the request line, headers, proxy, scripts, and body -- then
// resolves it against vars (see Resolve) using the variables known at parse
// time. ok is false for blocks that contain only comments/blank/
// variable-declaration lines -- those aren't requests and produce no
// issues.
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

				// prevent duplicated name metadata
				if key == "name" && nameDeclared && req.Name != value {
					issues = append(issues, Issue{
						Element:  "metadata:name",
						Severity: SeverityError,
						Message:  fmt.Sprintf("duplicated @name metadata: `%s` / `%s`", req.Name, value),
					})
				}
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

	req.Variables = localVars
	req.Lang = "lua"

	line := strings.TrimSpace(lines[i])
	if strings.HasPrefix(line, "<") {
		if !isScriptOpenLine(line, "<") {
			issues = append(issues, Issue{
				Element:  "script:pre",
				Severity: SeverityError,
				Message:  fmt.Sprintf("malformed pre-request script marker %q, expected \"< {%%}\"", line),
			})
			return req, issues, true
		}
		source, next, closed := scriptBlock(lines, i)
		if !closed {
			issues = append(issues, Issue{
				Element:  "script:pre",
				Severity: SeverityError,
				Message:  "unterminated \"< {%\" script block (missing closing \"%}\")",
			})
			return req, issues, true
		}
		issues = append(issues, checkScriptSyntax("script:pre", source)...)
		req.PreScript = source
		i = next
		for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
			i++
		}
		if i >= len(lines) {
			issues = append(issues, Issue{
				Element:  "request-line",
				Severity: SeverityError,
				Message:  "missing request line after pre-request script",
			})
			return req, issues, true
		}
		line = strings.TrimSpace(lines[i])
	}
	if strings.HasPrefix(line, ">") {
		issues = append(issues, Issue{
			Element:  "request-line",
			Severity: SeverityError,
			Message:  "unexpected \"> {%\" before the request line (post-request scripts go after headers/body, not before it)",
		})
		return req, issues, true
	}

	method, rawURL, proto, lineIssues := parseRequestLine(line)
	req.Method, req.RawURL, req.Proto = method, rawURL, proto
	issues = append(issues, lineIssues...)
	i++

	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		i++
		if line == "" {
			break
		}
		header, headerIssues := parseHeaderLine(line)
		if !hasError(headerIssues) {
			req.RawHeaders = append(req.RawHeaders, header)
		}
		issues = append(issues, headerIssues...)
	}

	rawProxy := localMetadata["proxy"]
	if rawProxy == "" {
		rawProxy = globalMetadata["proxy"]
	}
	req.RawProxy = rawProxy

	bodyLines, postScript, postIssues := extractPostScript(lines[i:])
	issues = append(issues, postIssues...)
	if postScript != "" {
		issues = append(issues, checkScriptSyntax("script:post", postScript)...)
		req.PostScript = postScript
	}
	req.RawBody = trimBody(bodyLines)

	vars := mergeVars(globalVars, localVars)
	resolved, resolveIssues := Resolve(req, vars)
	issues = append(issues, resolveIssues...)

	return resolved, issues, true
}

// Resolve interpolates req's still-templated fields (RawURL, RawHeaders,
// RawBody, RawProxy) against vars, returning a copy with URL, Headers,
// Body, and Proxy filled in, plus any issues found while doing so
// (undefined variables, or a URL/proxy that still isn't absolute once
// resolved). Method, Proto, and every other field are copied through
// unchanged.
//
// Analyze calls this once per block, using the variables known at parse
// time. A caller that runs scripts (the CLI's "run" command) calls it
// again immediately before actually sending a request, with whatever
// variables are current at that point -- which may include values an
// earlier request's post-request script just set -- so a request sees the
// most up-to-date variables available right before it's used, not just
// whatever was known when the file was first parsed.
func Resolve(req httpfile.Request, vars map[string]string) (httpfile.Request, []Issue) {
	var issues []Issue

	if req.RawURL != "" {
		resolvedURL, missing := interpolate.ApplyURL(req.RawURL, vars)
		req.URL = resolvedURL
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
			// A still-unresolved placeholder already explains why this
			// doesn't look absolute; don't pile on a second issue for it.
			issues = append(issues, Issue{
				Element:  "url",
				Severity: SeverityError,
				Message:  "URL must be absolute (missing scheme or host)",
			})
		}
	}

	req.Headers = nil
	for _, h := range req.RawHeaders {
		value, missing := interpolate.Apply(h.Value, vars)
		for _, name := range missing {
			issues = append(issues, undefinedVariableIssue("header:"+h.Name, name))
		}
		req.Headers = append(req.Headers, httpfile.Header{Name: h.Name, Value: value})
	}

	body, missing := interpolate.Apply(req.RawBody, vars)
	for _, name := range missing {
		issues = append(issues, undefinedVariableIssue("body", name))
	}
	body, fileIssues := spliceFileReferences(body)
	issues = append(issues, fileIssues...)
	req.Body = body

	resolvedProxy, proxyIssues := resolveProxy(req.RawProxy, vars)
	req.Proxy = resolvedProxy
	issues = append(issues, proxyIssues...)

	return req, issues
}

// isScriptOpenLine reports whether line (already trimmed) is exactly
// "<marker> {%", the opening of a script block.
func isScriptOpenLine(line, marker string) bool {
	return strings.TrimSpace(strings.TrimPrefix(line, marker)) == "{%"
}

// scriptBlock captures a script's source starting right after lines[openIdx]
// (which must already be a verified opening line) up to a line that is
// exactly "%}". next is the index of the first line after that closing
// line; closed is false if no closing line was found before EOF.
func scriptBlock(lines []string, openIdx int) (source string, next int, closed bool) {
	var src []string
	j := openIdx + 1
	for j < len(lines) {
		if strings.TrimSpace(lines[j]) == "%}" {
			return strings.Join(src, "\n"), j + 1, true
		}
		src = append(src, lines[j])
		j++
	}
	return strings.Join(src, "\n"), j, false
}

// extractPostScript looks for a "> {%" ... "%}" block anywhere in lines
// (the request's body region) and, if found, splits it out: bodyLines is
// everything before the block, script is its source. If no such block
// exists, bodyLines is lines unchanged and script is "".
func extractPostScript(lines []string) (bodyLines []string, script string, issues []Issue) {
	for idx, raw := range lines {
		if strings.TrimSpace(raw) != "> {%" {
			continue
		}
		source, next, closed := scriptBlock(lines, idx)
		if !closed {
			return lines, "", []Issue{{
				Element:  "script:post",
				Severity: SeverityError,
				Message:  "unterminated \"> {%\" script block (missing closing \"%}\")",
			}}
		}
		for _, trailing := range lines[next:] {
			if strings.TrimSpace(trailing) != "" {
				issues = append(issues, Issue{
					Element:  "script:post",
					Severity: SeverityWarning,
					Message:  "content after the post-request script is ignored",
				})
				break
			}
		}
		return lines[:idx], source, issues
	}
	return lines, "", nil
}

// checkScriptSyntax compiles (but never runs) a script's Lua source, so
// validate catches syntax errors without the side effects actually running
// it might have.
func checkScriptSyntax(element, source string) []Issue {
	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	defer L.Close()
	if _, err := L.LoadString(source); err != nil {
		return []Issue{{
			Element:  element,
			Severity: SeverityError,
			Message:  fmt.Sprintf("lua syntax error: %v", err),
		}}
	}
	return nil
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
		if value == "" {
			return []Issue{{Element: "metadata:lang", Severity: SeverityError, Message: "@lang requires a value"}}
		}
		if !strings.EqualFold(value, "lua") {
			return []Issue{{
				Element:  "metadata:lang",
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("unsupported scripting language %q; only \"lua\" is currently supported (falling back to lua)", value),
			}}
		}
		return nil
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

// parseRequestLine performs structural parsing of a request line only --
// splitting out the method, URL template, and proto, plus checks that
// don't depend on variable values (method standardness, proto format).
// Resolving {{var}} placeholders in the URL happens later, in Resolve.
func parseRequestLine(line string) (method, rawURL, proto string, issues []Issue) {
	fields := strings.Fields(line)
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

	if len(fields) == 3 && !protoPattern.MatchString(proto) {
		issues = append(issues, Issue{
			Element:  "proto",
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("unrecognized protocol version %q", proto),
		})
	}

	return method, rawURL, proto, issues
}

// parseHeaderLine performs structural parsing of a "Name: Value" header
// line -- Value is returned as its still-templated raw text; resolving any
// {{var}} placeholder in it happens later, in Resolve.
func parseHeaderLine(line string) (httpfile.Header, []Issue) {
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
	return httpfile.Header{Name: name, Value: strings.TrimSpace(rawValue)}, nil
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
		Message:  fmt.Sprintf("%s%q", undefinedVariablePrefix, name),
	}
}
