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
// value may itself contain "{{other}}" placeholders, which are expanded
// recursively against the same vars (so "@base = {{scheme}}://{{host}}"
// works as expected). A placeholder whose name isn't in vars is left
// untouched in the result and its name is added to missing, in
// first-occurrence order with duplicates removed -- including one found
// inside another variable's value. A placeholder whose expansion would
// loop back on itself ("@a = {{b}}", "@b = {{a}}") is likewise left
// untouched and the loop is described in cycles ("a -> b -> a").
func Apply(s string, vars map[string]string) (result string, missing, cycles []string) {
	return applyEscaped(s, vars, rawValue)
}

// ApplyURL is like Apply, but for URLs: a "{{name}}" placeholder that lands
// inside a query parameter's value is percent-encoded, so a value like
// "Hello again" becomes "Hello%20again" instead of a literal space
// corrupting the request line on the wire. Placeholders anywhere else --
// the scheme, host, or path -- are substituted raw, since a URL commonly
// builds its authority from a variable (e.g. "{{host}}/get") and that
// shouldn't be escaped the way a leaf value would be.
func ApplyURL(rawURL string, vars map[string]string) (result string, missing, cycles []string) {
	base, query, hasQuery := strings.Cut(rawURL, "?")
	if !hasQuery {
		return Apply(rawURL, vars)
	}

	resolvedBase, baseMissing, baseCycles := Apply(base, vars)
	resolvedQuery, queryMissing, queryCycles := applyToQueryValues(query, vars)

	return resolvedBase + "?" + resolvedQuery,
		dedupe(append(baseMissing, queryMissing...)),
		dedupe(append(baseCycles, queryCycles...))
}

// applyToQueryValues interpolates each "&"-separated "key=value" pair's
// value with percent-encoding; keys and valueless flags are substituted raw
// (they're structural, not data).
func applyToQueryValues(query string, vars map[string]string) (string, []string, []string) {
	params := strings.Split(query, "&")
	var missing, cycles []string
	for i, param := range params {
		key, value, hasValue := strings.Cut(param, "=")
		if !hasValue {
			resolved, m, c := Apply(param, vars)
			params[i] = resolved
			missing = append(missing, m...)
			cycles = append(cycles, c...)
			continue
		}
		resolvedValue, m, c := applyEscaped(value, vars, url.QueryEscape)
		missing = append(missing, m...)
		cycles = append(cycles, c...)
		params[i] = key + "=" + resolvedValue
	}
	return strings.Join(params, "&"), missing, cycles
}

func rawValue(s string) string { return s }

// expander carries the state of one Apply call: the variables, and the
// deduplicated missing-name and cycle reports collected as placeholders
// (including nested ones inside variable values) are expanded.
type expander struct {
	vars       map[string]string
	missing    []string
	cycles     []string
	seenMiss   map[string]bool
	seenCycles map[string]bool
}

func applyEscaped(s string, vars map[string]string, escape func(string) string) (result string, missing, cycles []string) {
	e := &expander{vars: vars, seenMiss: make(map[string]bool), seenCycles: make(map[string]bool)}
	result = e.expand(s, nil, escape)
	return result, e.missing, e.cycles
}

// expand substitutes the placeholders in s. stack is the chain of variable
// names whose values are currently being expanded (outermost first), used
// to detect a value that refers back to a variable already on the way in.
// escape is applied once to each fully-expanded top-level value, never to
// the intermediate nested ones, so a nested value is escaped exactly once.
func (e *expander) expand(s string, stack []string, escape func(string) string) string {
	return placeholderPattern.ReplaceAllStringFunc(s, func(match string) string {
		name := placeholderPattern.FindStringSubmatch(match)[1]
		v, ok := e.vars[name]
		if !ok {
			if !e.seenMiss[name] {
				e.seenMiss[name] = true
				e.missing = append(e.missing, name)
			}
			return match
		}
		for i, onStack := range stack {
			if onStack == name {
				path := strings.Join(append(append([]string(nil), stack[i:]...), name), " -> ")
				if !e.seenCycles[path] {
					e.seenCycles[path] = true
					e.cycles = append(e.cycles, path)
				}
				return match
			}
		}
		inner := append(append(make([]string, 0, len(stack)+1), stack...), name)
		return escape(e.expand(v, inner, rawValue))
	})
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
