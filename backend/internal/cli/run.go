package cli

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/cristianradulescu/httpfly/internal/client"
	"github.com/cristianradulescu/httpfly/internal/httpfile"
	"github.com/cristianradulescu/httpfly/internal/parser"
	"github.com/cristianradulescu/httpfly/internal/script"
	"github.com/cristianradulescu/httpfly/internal/state"
)

// requestOutcome is one request's result plus any error from its
// post-request script. A script error is tracked separately from
// Result.Err because, unlike a transport failure, the response was
// received successfully -- it should still be shown/counted, just flagged.
type requestOutcome struct {
	Result       client.Result
	ScriptErr    error
	DownloadPath string // non-empty if -download saved the body to this path instead of printing it
}

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
	download := fs.String("download", "", "save the response body to this file instead of printing it (only one request may be selected; the file's parent directory must already exist)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("run: expected exactly one .http file argument")
	}
	if silent && *jsonOutput {
		return fmt.Errorf("run: -silent and -json are mutually exclusive")
	}
	if silent && *download != "" {
		return fmt.Errorf("run: -silent and -download are mutually exclusive")
	}
	path := fs.Arg(0)

	dir, err := configDir()
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}

	envVars, err := resolveEnvVars(*envName)
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}

	persisted, err := state.Load(dir, *envName)
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Persisted client.global values take precedence over the environment's
	// (but not over a request's own local variables, which mergeVars inside
	// the parser still applies on top of whatever ParseWithEnv is given
	// here) -- so a token a script set earlier is what {{auth_token}}
	// actually resolves to.
	file, err := parser.ParseWithEnv(f, mergeVars(envVars, persisted))
	if err != nil {
		return err
	}

	requests, err := selectRequests(file.Requests, *name)
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	if *download != "" && len(requests) != 1 {
		return fmt.Errorf("run: -download requires selecting exactly one request (file has %d; use -name to pick one)", len(requests))
	}

	global := script.NewGlobalState(dir, *envName, persisted)

	c := client.New()
	var failed int
	var outcomes []requestOutcome
	for _, req := range requests {
		outcome := sendWithScripts(c, req, file.Variables, global)
		if *download != "" && outcome.Result.Err == nil {
			if err := os.WriteFile(*download, outcome.Result.Body, 0o644); err != nil {
				return fmt.Errorf("run: -download: %w", err)
			}
			outcome.DownloadPath = *download
		}
		switch {
		case *jsonOutput:
			outcomes = append(outcomes, outcome)
		case silent:
			if outcome.Result.Err == nil {
				stdout.Write(outcome.Result.Body)
			}
		default:
			printResult(stdout, outcome.Result, verbose, outcome.DownloadPath)
			if outcome.ScriptErr != nil {
				fmt.Fprintf(stdout, "post-request script error: %v\n\n", outcome.ScriptErr)
			}
		}
		if outcome.Result.Err != nil || outcome.ScriptErr != nil {
			failed++
		}
	}

	if *jsonOutput {
		if err := printJSONResults(stdout, outcomes, verbose); err != nil {
			return fmt.Errorf("run: %w", err)
		}
	}

	if failed > 0 {
		if *jsonOutput {
			// The JSON array is already on stdout, complete and parseable,
			// with each failed request's own "error"/"script_error" field --
			// printing a summary here too would risk corrupting that output
			// for a tool that merges stdout and stderr. Exit code alone
			// signals failure.
			return ErrSilent
		}
		return fmt.Errorf("run: %d of %d request(s) failed", failed, len(requests))
	}
	return nil
}

// sendWithScripts runs req's pre-request script (if any -- which may set
// persisted client.global values), re-resolves req's "{{var}}" placeholders
// against the current variables (base, overridden by whatever's in global
// right now -- including anything an earlier request in this same run just
// set -- overridden by req's own local variables), sends it, and runs its
// post-request script (if any) once a response is received successfully.
//
// Re-resolving here rather than trusting the URL/headers/body Analyze
// already produced is what makes a value set by one request's post-request
// script available to a later request, whether that's a later block in the
// same "httpfly run" or a request in a separate, later invocation.
func sendWithScripts(c *client.Client, req httpfile.Request, baseVars map[string]string, global *script.GlobalState) requestOutcome {
	if req.PreScript != "" {
		if err := script.RunPreScript(req.PreScript, global); err != nil {
			return requestOutcome{Result: client.Result{Request: req, Err: fmt.Errorf("pre-request script: %w", err)}}
		}
	}

	vars := mergeVars(mergeVars(baseVars, global.Vars()), req.Variables)
	resolved, resolveIssues := parser.Resolve(req, vars)
	if err := issuesAsFatal(resolveIssues, "cannot send"); err != nil {
		return requestOutcome{Result: client.Result{Request: resolved, Err: err}}
	}

	result := c.Send(context.Background(), resolved)

	var scriptErr error
	if result.Err == nil && req.PostScript != "" {
		scriptErr = script.RunPostScript(req.PostScript, result, global)
	}
	return requestOutcome{Result: result, ScriptErr: scriptErr}
}

// issuesAsFatal turns resolution issues into one error, prefixed by verb,
// if req isn't safe to actually use: an outright error (e.g. an invalid
// proxy URL), or a variable that's still undefined -- Analyze only ever
// warns about that (a script might set it before the request is used),
// but by the time a caller is about to send it, or export it as a final
// artifact like a curl command, there's no more "might" left, so httpfly
// treats it as fatal instead of silently producing something with a
// literal "{{name}}" in it.
func issuesAsFatal(issues []parser.Issue, verb string) error {
	var msgs []string
	for _, issue := range issues {
		if issue.Severity == parser.SeverityError || parser.IsUndefinedVariableIssue(issue) {
			msgs = append(msgs, fmt.Sprintf("%s: %s", issue.Element, issue.Message))
		}
	}
	if len(msgs) == 0 {
		return nil
	}
	return fmt.Errorf("%s: %s", verb, strings.Join(msgs, "; "))
}

func printResult(w io.Writer, r client.Result, verbose bool, downloadPath string) {
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

	switch {
	case downloadPath != "":
		fmt.Fprintf(w, "\n[saved %d bytes to %s]\n", len(r.Body), downloadPath)
	case len(r.Body) > 0:
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
