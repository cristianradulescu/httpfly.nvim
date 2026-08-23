// Package curl converts a bash-style "curl ..." command (e.g. a browser's
// "Copy as cURL" output) into an httpfly request. Windows cmd.exe and
// PowerShell's own "Copy as cURL" variants (different quoting/continuation
// rules) aren't supported -- only bash-style.
package curl

import (
	"fmt"
	"strings"
)

// Tokenize splits a bash-style command line into argv-style tokens,
// honoring single quotes (fully literal until the matching quote --
// nothing inside is interpreted, including embedded double quotes,
// "=", ";", "$", etc.), double quotes (backslash escapes "\", "$", "`",
// """, and a line-continuation newline), unquoted backslash-escaping of
// the next character, and "\" followed by a newline outside quotes as a
// line continuation. Adjacent quoted/unquoted spans with no whitespace
// between them join into a single token, matching shell semantics.
func Tokenize(s string) ([]string, error) {
	var tokens []string
	var cur strings.Builder
	inToken := false

	flush := func() {
		if inToken {
			tokens = append(tokens, cur.String())
			cur.Reset()
			inToken = false
		}
	}

	i, n := 0, len(s)
	for i < n {
		c := s[i]
		switch {
		case c == '\\' && i+1 < n && s[i+1] == '\n':
			i += 2 // unquoted line continuation: drop both characters
		case c == '\\' && i+2 < n && s[i+1] == '\r' && s[i+2] == '\n':
			i += 3
		case c == '\'':
			inToken = true
			j := i + 1
			for j < n && s[j] != '\'' {
				j++
			}
			if j >= n {
				return nil, fmt.Errorf("unterminated single-quoted string starting at byte %d", i)
			}
			cur.WriteString(s[i+1 : j])
			i = j + 1
		case c == '"':
			inToken = true
			j := i + 1
			for j < n && s[j] != '"' {
				if s[j] == '\\' && j+1 < n && strings.ContainsRune(`\$`+"`"+`"`+"\n", rune(s[j+1])) {
					if s[j+1] != '\n' {
						cur.WriteByte(s[j+1])
					}
					j += 2
					continue
				}
				cur.WriteByte(s[j])
				j++
			}
			if j >= n {
				return nil, fmt.Errorf("unterminated double-quoted string starting at byte %d", i)
			}
			i = j + 1
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			flush()
			i++
		case c == '\\' && i+1 < n:
			inToken = true
			cur.WriteByte(s[i+1])
			i += 2
		default:
			inToken = true
			cur.WriteByte(c)
			i++
		}
	}
	flush()
	return tokens, nil
}
