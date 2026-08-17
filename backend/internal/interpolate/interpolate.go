// Package interpolate substitutes "{{name}}" placeholders with variable
// values. It doesn't care where the values come from -- a file-scoped
// declaration, an environment, a script -- callers just hand it a resolved
// map[string]string.
package interpolate

import "regexp"

var placeholderPattern = regexp.MustCompile(`\{\{\s*([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)

// Apply replaces every "{{name}}" placeholder in s with vars[name]. A
// placeholder whose name isn't in vars is left untouched in the result and
// its name is added to missing, in first-occurrence order with duplicates
// removed.
func Apply(s string, vars map[string]string) (result string, missing []string) {
	seen := make(map[string]bool)
	result = placeholderPattern.ReplaceAllStringFunc(s, func(match string) string {
		name := placeholderPattern.FindStringSubmatch(match)[1]
		if v, ok := vars[name]; ok {
			return v
		}
		if !seen[name] {
			seen[name] = true
			missing = append(missing, name)
		}
		return match
	})
	return result, missing
}
