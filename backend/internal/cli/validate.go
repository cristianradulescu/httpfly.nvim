package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/cristianradulescu/httpfly/internal/parser"
)

func validateCommand(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stdout)
	name := fs.String("name", "", "only report on the request with this @name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("validate: expected exactly one .http file argument")
	}
	path := fs.Arg(0)

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	result, err := parser.Analyze(f)
	if err != nil {
		return err
	}

	if *name != "" {
		result, err = filterResult(result, *name)
		if err != nil {
			return fmt.Errorf("validate: %w", err)
		}
	}

	printValidationReport(stdout, path, result)

	if result.HasErrors() {
		return fmt.Errorf("validate: %s has errors", path)
	}
	return nil
}

// filterResult keeps only the block whose request is named name. Blocks
// with a missing/invalid @name (an error the report should still surface)
// never match and would otherwise vanish silently, so unmatched names
// return an error instead of an empty report.
func filterResult(result *parser.Result, name string) (*parser.Result, error) {
	for _, block := range result.Blocks {
		if block.Request.Name == name {
			return &parser.Result{
				Variables:    result.Variables,
				GlobalIssues: result.GlobalIssues,
				Blocks:       []parser.BlockResult{block},
			}, nil
		}
	}
	return nil, fmt.Errorf("no request named %q", name)
}

func printValidationReport(w io.Writer, path string, result *parser.Result) {
	var errCount, warnCount int

	if len(result.Variables) > 0 {
		names := make([]string, 0, len(result.Variables))
		for name := range result.Variables {
			names = append(names, name)
		}
		sort.Strings(names)
		fmt.Fprintf(w, "%s: %d global variable(s) declared: %s\n\n", path, len(names), strings.Join(names, ", "))
	}

	if len(result.GlobalIssues) > 0 {
		fmt.Fprintf(w, "%s (global):\n", path)
		for _, issue := range result.GlobalIssues {
			fmt.Fprintf(w, "  [%s] %s: %s\n", issue.Severity, issue.Element, issue.Message)
			switch issue.Severity {
			case parser.SeverityError:
				errCount++
			case parser.SeverityWarning:
				warnCount++
			}
		}
	}

	for _, block := range result.Blocks {
		label := block.Request.Name
		if label == "" {
			label = fmt.Sprintf("block %d", block.Index)
		}
		fmt.Fprintf(w, "%s (%s):\n", path, label)

		if len(block.Issues) == 0 {
			fmt.Fprintln(w, "  ok")
		}
		for _, issue := range block.Issues {
			fmt.Fprintf(w, "  [%s] %s: %s\n", issue.Severity, issue.Element, issue.Message)
			switch issue.Severity {
			case parser.SeverityError:
				errCount++
			case parser.SeverityWarning:
				warnCount++
			}
		}
	}

	fmt.Fprintf(w, "\n%d request(s) checked, %d error(s), %d warning(s)\n", len(result.Blocks), errCount, warnCount)
}
