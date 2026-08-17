// Package parser turns .http file content into httpfile.File values.
package parser

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/cristianradulescu/httpfly/internal/httpfile"
)

const separator = "###"

// Parse reads r and returns the requests it defines, or an error describing
// every validation error found (see Analyze for a report that also includes
// warnings and succeeds despite errors).
func Parse(r io.Reader) (*httpfile.File, error) {
	result, err := Analyze(r)
	if err != nil {
		return nil, err
	}

	if result.HasErrors() {
		var msgs []string
		for _, b := range result.Blocks {
			for _, issue := range b.Issues {
				if issue.Severity == SeverityError {
					msgs = append(msgs, fmt.Sprintf("request %d: %s: %s", b.Index, issue.Element, issue.Message))
				}
			}
		}
		return nil, fmt.Errorf("parser: %s", strings.Join(msgs, "; "))
	}

	f := &httpfile.File{Variables: result.Variables}
	for _, b := range result.Blocks {
		f.Requests = append(f.Requests, b.Request)
	}
	return f, nil
}

// Analyze reads r and validates every request block, collecting issues
// instead of stopping at the first one. It only returns an error for
// unrecoverable problems reading r.
func Analyze(r io.Reader) (*Result, error) {
	blocks, err := splitBlocks(r)
	if err != nil {
		return nil, fmt.Errorf("parser: %w", err)
	}

	vars := collectVariables(blocks)

	result := &Result{Variables: vars}
	index := 0
	for _, block := range blocks {
		req, issues, ok := validateBlock(block, vars)
		if !ok {
			continue
		}
		index++
		result.Blocks = append(result.Blocks, BlockResult{Index: index, Request: req, Issues: issues})
	}
	return result, nil
}

// splitBlocks splits r into ###-delimited segments, including whatever
// precedes the first separator (or the whole file, if there is none) --
// that segment carries no request but may declare file-scoped variables.
func splitBlocks(r io.Reader) ([][]string, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var blocks [][]string
	var current []string

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == separator {
			blocks = append(blocks, current)
			current = nil
			continue
		}
		current = append(current, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	blocks = append(blocks, current)
	return blocks, nil
}

// parseMetadata recognizes "# @key value" comment lines.
func parseMetadata(line string) (key, value string, ok bool) {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "#"))
	if !strings.HasPrefix(rest, "@") {
		return "", "", false
	}
	rest = strings.TrimPrefix(rest, "@")
	parts := strings.SplitN(rest, " ", 2)
	key = parts[0]
	if len(parts) == 2 {
		value = strings.TrimSpace(parts[1])
	}
	return key, value, true
}

var variablePattern = regexp.MustCompile(`^@([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$`)

// parseVariableDef recognizes "@name = value" lines (file-scoped variable
// declarations -- unlike metadata, these aren't comments).
func parseVariableDef(line string) (name, value string, ok bool) {
	m := variablePattern.FindStringSubmatch(line)
	if m == nil {
		return "", "", false
	}
	return m[1], strings.TrimSpace(m[2]), true
}

// collectVariables scans the leading blank/comment/variable-declaration
// section of every block for "@name = value" lines. Declarations anywhere in
// the file are treated as file-scoped, in declaration order, later ones
// overriding earlier ones with the same name.
func collectVariables(blocks [][]string) map[string]string {
	vars := make(map[string]string)
	for _, lines := range blocks {
		for _, raw := range lines {
			line := strings.TrimSpace(raw)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			name, value, ok := parseVariableDef(line)
			if !ok {
				break
			}
			vars[name] = value
		}
	}
	return vars
}

// trimBody drops leading/trailing blank lines while preserving indentation
// and blank lines within the body itself.
func trimBody(lines []string) string {
	start, end := 0, len(lines)
	for start < end && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return strings.Join(lines[start:end], "\n")
}
