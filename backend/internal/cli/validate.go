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

	printValidationReport(stdout, path, result)

	if result.HasErrors() {
		return fmt.Errorf("validate: %s has errors", path)
	}
	return nil
}

func printValidationReport(w io.Writer, path string, result *parser.Result) {
	var errCount, warnCount int

	if len(result.Variables) > 0 {
		names := make([]string, 0, len(result.Variables))
		for name := range result.Variables {
			names = append(names, name)
		}
		sort.Strings(names)
		fmt.Fprintf(w, "%s: %d variable(s) declared: %s\n\n", path, len(names), strings.Join(names, ", "))
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
