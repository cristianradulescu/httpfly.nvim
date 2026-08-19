package cli

import (
	"context"
	"crypto/tls"
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
	name := fs.String("name", "", "only send the request with this @name")
	var silent bool
	fs.BoolVar(&silent, "silent", false, "print only response bodies, nothing else (like curl -s)")
	fs.BoolVar(&silent, "s", false, "shorthand for -silent")
	jsonOutput := fs.Bool("json", false, "print results as a JSON array instead of plain text")
	var verbose bool
	fs.BoolVar(&verbose, "verbose", false, "also print TLS connection details (version, cipher, peer certificate)")
	fs.BoolVar(&verbose, "v", false, "shorthand for -verbose")
	envName := fs.String("env", "", "apply variables from the named environment in httpfly.env.json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("run: expected exactly one .http file argument")
	}
	if silent && *jsonOutput {
		return fmt.Errorf("run: -silent and -json are mutually exclusive")
	}
	path := fs.Arg(0)

	envVars, err := resolveEnvVars(path, *envName)
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	file, err := parser.ParseWithEnv(f, envVars)
	if err != nil {
		return err
	}

	requests, err := selectRequests(file.Requests, *name)
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}

	c := client.New()
	var failed int
	var results []client.Result
	for _, req := range requests {
		result := c.Send(context.Background(), req)
		switch {
		case *jsonOutput:
			results = append(results, result)
		case silent:
			if result.Err == nil {
				stdout.Write(result.Body)
			}
		default:
			printResult(stdout, result, verbose)
		}
		if result.Err != nil {
			failed++
		}
	}

	if *jsonOutput {
		if err := printJSONResults(stdout, results, verbose); err != nil {
			return fmt.Errorf("run: %w", err)
		}
	}

	if failed > 0 {
		return fmt.Errorf("run: %d of %d request(s) failed to send", failed, len(requests))
	}
	return nil
}

func printResult(w io.Writer, r client.Result, verbose bool) {
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

	if r.FinalURL != "" && r.FinalURL != r.Request.URL {
		fmt.Fprintf(w, "redirected to: %s\n", r.FinalURL)
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

	if verbose {
		printTLSInfo(w, r.TLS)
	}

	if len(r.Body) > 0 {
		fmt.Fprintf(w, "\n%s\n", r.Body)
	}

	fmt.Fprintln(w)
}

func printTLSInfo(w io.Writer, state *tls.ConnectionState) {
	if state == nil {
		fmt.Fprintln(w, "\ntls: none (plain HTTP)")
		return
	}
	fmt.Fprintln(w, "\ntls:")
	fmt.Fprintf(w, "  version: %s\n", tls.VersionName(state.Version))
	fmt.Fprintf(w, "  cipher: %s\n", tls.CipherSuiteName(state.CipherSuite))
	if state.NegotiatedProtocol != "" {
		fmt.Fprintf(w, "  alpn: %s\n", state.NegotiatedProtocol)
	}
	if len(state.PeerCertificates) > 0 {
		cert := state.PeerCertificates[0]
		fmt.Fprintf(w, "  peer certificate: subject=%s issuer=%s expires=%s\n",
			cert.Subject, cert.Issuer, cert.NotAfter.Format(time.RFC3339))
	}
}
