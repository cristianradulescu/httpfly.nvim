package curl

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"github.com/cristianradulescu/httpfly/internal/httpfile"
)

// flagsWithValue are the curl flags this package understands that consume
// the following token as their value (or an "=value" suffix on a long
// flag). Anything not listed here is treated as taking no value.
var flagsWithValue = map[string]bool{
	"url": true, "X": true, "request": true,
	"H": true, "header": true,
	"d": true, "data": true, "data-ascii": true, "data-binary": true, "data-raw": true, "data-urlencode": true,
	"F": true, "form": true,
	"b": true, "cookie": true,
	"A": true, "user-agent": true,
	"e": true, "referer": true,
	"u": true, "user": true,
	"x": true, "proxy": true,
	"o": true, "output": true,
}

// ignoredNoValueFlags take no value and have no effect on the request
// itself, so they're dropped without a warning -- they're either
// meaningless for httpfly (e.g. --compressed, which Go's transport already
// handles transparently) or already httpfly's default behavior (-L, which
// follows redirects like httpfly's client already does).
var ignoredNoValueFlags = map[string]bool{
	"compressed": true,
	"L":          true, "location": true,
	"s": true, "silent": true,
	"S": true, "show-error": true,
	"v": true, "verbose": true,
	"http1.1": true, "http2": true,
	"O": true,
}

// Parse converts a bash-style "curl ..." command into an httpfly request.
// Only Method, URL, Headers, Body, and Proxy are populated -- Name and
// Proto are the caller's responsibility. warnings describes any flag that
// was recognized but ignored (unsupported, or with no httpfly equivalent);
// they don't prevent a request from being produced.
func Parse(cmd string) (httpfile.Request, []string, error) {
	tokens, err := Tokenize(cmd)
	if err != nil {
		return httpfile.Request{}, nil, err
	}
	tokens, err = stripLeadingCurl(tokens)
	if err != nil {
		return httpfile.Request{}, nil, err
	}

	var req httpfile.Request
	var explicitMethod, rawURL string
	var dataParts []string
	var fileDataPath string
	var formParts []formPart
	var warnings []string
	positionalOnly := false

	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]

		if tok == "--" && !positionalOnly {
			positionalOnly = true
			continue
		}
		if positionalOnly || tok == "" || tok[0] != '-' || tok == "-" {
			if rawURL == "" {
				rawURL = tok
			} else {
				warnings = append(warnings, fmt.Sprintf("ignored extra positional argument %q", tok))
			}
			continue
		}

		name, inlineValue, hasInline := splitLongFlag(tok)

		var value string
		if flagsWithValue[name] {
			if hasInline {
				value = inlineValue
			} else {
				i++
				if i >= len(tokens) {
					return httpfile.Request{}, warnings, fmt.Errorf("flag %q expects a value but none was given", tok)
				}
				value = tokens[i]
			}
		}

		switch name {
		case "url":
			rawURL = value
		case "X", "request":
			explicitMethod = value
		case "H", "header":
			hName, hValue, found := strings.Cut(value, ":")
			if !found {
				warnings = append(warnings, fmt.Sprintf("ignored malformed header %q (expected \"Name: Value\")", value))
				continue
			}
			addHeader(&req, &warnings, hName, strings.TrimSpace(hValue))
		case "d", "data", "data-ascii", "data-binary":
			if path, ok := strings.CutPrefix(value, "@"); ok {
				if fileDataPath != "" || len(dataParts) > 0 {
					warnings = append(warnings, fmt.Sprintf("ignored \"%s %s\" (a file data part can't be combined with other -d/--data values)", tok, value))
					continue
				}
				fileDataPath = path
				continue
			}
			if fileDataPath != "" {
				warnings = append(warnings, fmt.Sprintf("ignored \"%s %s\" (a file data part can't be combined with other -d/--data values)", tok, value))
				continue
			}
			dataParts = append(dataParts, value)
		case "data-raw":
			if fileDataPath != "" {
				warnings = append(warnings, fmt.Sprintf("ignored \"%s %s\" (a file data part can't be combined with other -d/--data values)", tok, value))
				continue
			}
			dataParts = append(dataParts, value)
		case "data-urlencode":
			if strings.Contains(value, "@") {
				warnings = append(warnings, fmt.Sprintf("ignored \"%s %s\" (reading data from a file is not supported)", tok, value))
				continue
			}
			if fileDataPath != "" {
				warnings = append(warnings, fmt.Sprintf("ignored \"%s %s\" (a file data part can't be combined with other -d/--data values)", tok, value))
				continue
			}
			dataParts = append(dataParts, dataURLEncode(value))
		case "F", "form":
			part, err := parseFormField(value)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("ignored malformed \"%s %s\": %v", tok, value, err))
				continue
			}
			formParts = append(formParts, part)
		case "b", "cookie":
			addHeader(&req, &warnings, "Cookie", value)
		case "A", "user-agent":
			addHeader(&req, &warnings, "User-Agent", value)
		case "e", "referer":
			addHeader(&req, &warnings, "Referer", value)
		case "u", "user":
			addHeader(&req, &warnings, "Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(value)))
		case "x", "proxy":
			req.Proxy = value
		case "I", "head":
			if explicitMethod == "" {
				explicitMethod = "HEAD"
			}
		case "k", "insecure":
			warnings = append(warnings, "ignored \"-k/--insecure\" (skipping TLS verification has no httpfly equivalent)")
		case "o", "output":
			warnings = append(warnings, fmt.Sprintf("ignored \"-o/--output %s\" (httpfly always prints the response)", value))
		default:
			if ignoredNoValueFlags[name] {
				continue
			}
			if flagsWithValue[name] {
				warnings = append(warnings, fmt.Sprintf("ignored unrecognized flag \"%s %s\"", tok, value))
			} else {
				warnings = append(warnings, fmt.Sprintf("ignored unrecognized flag %q", tok))
			}
		}
	}

	if rawURL == "" {
		return httpfile.Request{}, warnings, fmt.Errorf("no URL found in curl command")
	}
	if _, err := url.Parse(rawURL); err != nil {
		return httpfile.Request{}, warnings, fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}
	req.URL = rawURL

	if len(formParts) > 0 && (len(dataParts) > 0 || fileDataPath != "") {
		warnings = append(warnings, "ignored -d/--data (mixing it with -F/--form is not supported; -F was used)")
		dataParts, fileDataPath = nil, ""
	}

	switch {
	case explicitMethod != "":
		req.Method = strings.ToUpper(explicitMethod)
	case len(dataParts) > 0 || fileDataPath != "" || len(formParts) > 0:
		req.Method = "POST"
	default:
		req.Method = "GET"
	}
	switch {
	case len(formParts) > 0:
		req.Body = formatMultipartBody(formParts, multipartBoundary)
		addHeader(&req, &warnings, "Content-Type", "multipart/form-data; boundary="+multipartBoundary)
	case fileDataPath != "":
		req.Body = "< " + fileDataPath
	case len(dataParts) > 0:
		req.Body = strings.Join(dataParts, "&")
	}

	return req, warnings, nil
}

// stripLeadingCurl drops the leading "curl" token curl commands always
// start with.
func stripLeadingCurl(tokens []string) ([]string, error) {
	if len(tokens) == 0 || !strings.EqualFold(tokens[0], "curl") {
		return nil, fmt.Errorf("expected input to start with \"curl\"")
	}
	return tokens[1:], nil
}

// splitLongFlag strips a flag's leading dashes and, for a long flag
// ("--flag=value"), splits out an inline value.
func splitLongFlag(tok string) (name, inlineValue string, hasInline bool) {
	trimmed := strings.TrimLeft(tok, "-")
	if strings.HasPrefix(tok, "--") {
		if idx := strings.Index(trimmed, "="); idx >= 0 {
			return trimmed[:idx], trimmed[idx+1:], true
		}
	}
	return trimmed, "", false
}

// addHeader appends a header, skipping (with a warning) one whose name or
// value contains a raw newline -- writing that straight into an .http
// file's "Name: Value" line would corrupt the file's structure.
func addHeader(req *httpfile.Request, warnings *[]string, name, value string) {
	name = strings.TrimSpace(name)
	if strings.ContainsAny(name, "\r\n") || strings.ContainsAny(value, "\r\n") {
		*warnings = append(*warnings, fmt.Sprintf("ignored header %q: value contains an embedded newline", name))
		return
	}
	req.Headers = append(req.Headers, httpfile.Header{Name: name, Value: value})
}

// dataURLEncode implements --data-urlencode's "[name=]content" form,
// percent-encoding content (or the whole value, if it's not name=value).
func dataURLEncode(raw string) string {
	name, value, found := strings.Cut(raw, "=")
	if !found {
		return url.QueryEscape(raw)
	}
	return name + "=" + url.QueryEscape(value)
}
