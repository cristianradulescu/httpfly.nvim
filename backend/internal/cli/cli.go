// Package cli wires httpfly's command-line arguments to its subcommands.
package cli

import (
	"errors"
	"fmt"
	"io"
)

// ErrSilent marks a returned error whose message must not be printed --
// used when the caller (currently just "run -json") has already written
// its complete, parseable output to stdout and a human-readable summary
// alongside it would risk corrupting that output for a tool reading it
// (e.g. one that merges stdout and stderr). The exit code still reflects
// failure; wrap it with fmt.Errorf's %w or return it directly.
var ErrSilent = errors.New("")

// Run dispatches args to a subcommand, reading from stdin and writing
// normal/error output to stdout/stderr respectively.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stderr)
		return fmt.Errorf("missing command")
	}

	switch args[0] {
	case "run":
		return runCommand(args[1:], stdout, stderr)
	case "validate":
		return validateCommand(args[1:], stdout, stderr)
	case "convert":
		return convertCommand(args[1:], stdin, stdout, stderr)
	case "version", "-version", "--version":
		fmt.Fprintf(stdout, "httpfly %s\n", EffectiveVersion())
		return nil
	case "help", "-h", "--help":
		printUsage(stdout)
		return nil
	default:
		printUsage(stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: httpfly <command> [arguments]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  run [-name X] [-env E] [-s | -json] [-v] [-download F] [-timeout D] <file.http>   send the requests in an .http file and print their responses")
	fmt.Fprintln(w, "  validate [-name X] [-env E] <file.http>                report per-block validation issues in an .http file")
	fmt.Fprintln(w, "  convert from-curl [-name X] [-file F]                  convert a bash-style curl command (stdin, or -file) to .http")
	fmt.Fprintln(w, "  convert to-curl [-name X] [-env E] <file.http>         convert an .http request to a multi-line bash-style curl command")
	fmt.Fprintln(w, "  version                                                print the httpfly version")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "-name restricts either command to the single request named X (via \"# @name X\" or \"### X\").")
	fmt.Fprintln(w, "-env applies the named environment's variables from http-client.env.json (plus an optional")
	fmt.Fprintln(w, "  http-client.private.env.json overlay for values you don't want committed), found in the")
	fmt.Fprintln(w, "  current working directory (not the .http file's own directory) -- overriding the")
	fmt.Fprintln(w, "  file's own global variables but not a request's local ones.")
	fmt.Fprintln(w, "-s/-silent (run only) prints only response bodies, nothing else -- like curl -s.")
	fmt.Fprintln(w, "-json (run only) prints a JSON array of {name, request, response|error, duration_ms}.")
	fmt.Fprintln(w, "-v/-verbose (run only) also prints TLS connection details (version, cipher, peer certificate).")
	fmt.Fprintln(w, "-download F (run only) saves the response body to file F instead of printing it -- requires")
	fmt.Fprintln(w, "  selecting exactly one request (via -name, or a file with only one), and F's parent")
	fmt.Fprintln(w, "  directory must already exist. Compatible with -json (adds a \"download_path\" field and")
	fmt.Fprintln(w, "  empties \"body\"); mutually exclusive with -s/-silent.")
	fmt.Fprintln(w, "-timeout D (run only) sets the per-request timeout (connection, request, and reading the whole")
	fmt.Fprintln(w, "  response body), as a duration like 30s or 2m; 0 disables it. Default 30s.")
	fmt.Fprintln(w, "A redirected request's final URL is always noted, in either output format.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "\"< {% ... %}\" and \"> {% ... %}\" blocks run as Lua pre-/post-request scripts. They can")
	fmt.Fprintln(w, "read/write persisted variables via client.global:get/set(name[, value]) -- saved to")
	fmt.Fprintln(w, "  .httpfly/state.json in the current working directory, so a later, separate run reuses them.")
}
