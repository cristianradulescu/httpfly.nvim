// Package interpolate substitutes "{{name}}" placeholders with variable
// values. It doesn't care where the values come from -- a file-scoped
// declaration, an environment, a script -- callers just hand it a resolved
// map[string]string.
package interpolate

import (
	"net/url"
	"regexp"
	"strings"
)

var placeholderPattern = regexp.MustCompile(`\{\{\s*([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)

// Apply replaces every "{{name}}" placeholder in s with vars[name]. A
// placeholder whose name isn't in vars is left untouched in the result and
// its name is added to missing, in first-occurrence order with duplicates
// removed.
func Apply(s string, vars map[string]string) (result string, missing []string) {
	return applyEscaped(s, vars, rawValue)
}

// ApplyURL is like Apply, but for URLs: a "{{name}}" placeholder that lands
// inside a query parameter's value is percent-encoded, so a value like
// "Hello again" becomes "Hello%20again" instead of a literal space
// corrupting the request line on the wire. Placeholders anywhere else --
// the scheme, host, or path -- are substituted raw, since a URL commonly
// builds its authority from a variable (e.g. "{{host}}/get") and that
// shouldn't be escaped the way a leaf value would be.
func ApplyURL(rawURL string, vars map[string]string) (result string, missing []string) {
	base, query, hasQuery := strings.Cut(rawURL, "?")
	if !hasQuery {
		return Apply(rawURL, vars)
	}

	resolvedBase, baseMissing := Apply(base, vars)
	resolvedQuery, queryMissing := applyToQueryValues(query, vars)

	return resolvedBase + "?" + resolvedQuery, dedupe(append(baseMissing, queryMissing...))
}

// applyToQueryValues interpolates each "&"-separated "key=value" pair's
// value with percent-encoding; keys and valueless flags are substituted raw
// (they're structural, not data).
func applyToQueryValues(query string, vars map[string]string) (string, []string) {
	params := strings.Split(query, "&")
	var missing []string
	for i, param := range params {
		key, value, hasValue := strings.Cut(param, "=")
		if !hasValue {
			resolved, m := Apply(param, vars)
			params[i] = resolved
			missing = append(missing, m...)
			continue
		}
		resolvedValue, m := applyEscaped(value, vars, url.QueryEscape)
		missing = append(missing, m...)
		params[i] = key + "=" + resolvedValue
	}
	return strings.Join(params, "&"), missing
}

func rawValue(s string) string { return s }

func applyEscaped(s string, vars map[string]string, escape func(string) string) (result string, missing []string) {
	seen := make(map[string]bool)
	result = placeholderPattern.ReplaceAllStringFunc(s, func(match string) string {
		name := placeholderPattern.FindStringSubmatch(match)[1]
		if v, ok := vars[name]; ok {
			return escape(v)
		}
		if !seen[name] {
			seen[name] = true
			missing = append(missing, name)
		}
		return match
	})
	return result, missing
}

func dedupe(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(items))
	var out []string
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}
