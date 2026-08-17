package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/cristianradulescu/httpfly/internal/client"
	"github.com/cristianradulescu/httpfly/internal/parser"
)

func runCommand(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stdout)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("run: expected exactly one .http file argument")
	}

	f, err := os.Open(fs.Arg(0))
	if err != nil {
		return err
	}
	defer f.Close()

	file, err := parser.Parse(f)
	if err != nil {
		return err
	}

	c := client.New()
	var failed int
	for _, req := range file.Requests {
		result := c.Send(context.Background(), req)
		printResult(stdout, result)
		if result.Err != nil {
			failed++
		}
	}

	if failed > 0 {
		return fmt.Errorf("run: %d of %d request(s) failed to send", failed, len(file.Requests))
	}
	return nil
}

func printResult(w io.Writer, r client.Result) {
	label := r.Request.Name
	if label == "" {
		label = r.Request.Method + " " + r.Request.URL
	}
	fmt.Fprintf(w, "=== %s ===\n", label)
	fmt.Fprintf(w, "%s %s %s\n", r.Request.Method, r.Request.URL, r.Request.Proto)
	for _, h := range r.Request.Headers {
		fmt.Fprintf(w, "%s: %s\n", h.Name, h.Value)
	}
	if r.Request.Body != "" {
		fmt.Fprintf(w, "\n%s\n", r.Request.Body)
	}
	fmt.Fprintln(w)

	if r.Err != nil {
		fmt.Fprintf(w, "error: %v\n\n", r.Err)
		return
	}

	fmt.Fprintf(w, "%s (%s)\n", r.Status, r.Duration.Round(time.Millisecond))
	names := make([]string, 0, len(r.Headers))
	for name := range r.Headers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, v := range r.Headers[name] {
			fmt.Fprintf(w, "%s: %s\n", name, v)
		}
	}
	if len(r.Body) > 0 {
		fmt.Fprintf(w, "\n%s\n", r.Body)
	}
	fmt.Fprintln(w)
}
