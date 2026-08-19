// Package cli wires httpfly's command-line arguments to its subcommands.
package cli

import (
	"fmt"
	"io"
)

// Run dispatches args to a subcommand, writing normal and error output to
// stdout/stderr respectively.
func Run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stderr)
		return fmt.Errorf("missing command")
	}

	switch args[0] {
	case "run":
		return runCommand(args[1:], stdout)
	case "validate":
		return validateCommand(args[1:], stdout)
	case "version", "-version", "--version":
		fmt.Fprintf(stdout, "httpfly %s\n", Version)
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
	fmt.Fprintln(w, "  run [-name X] [-env E] [-s | -json] [-v] <file.http>   send the requests in an .http file and print their responses")
	fmt.Fprintln(w, "  validate [-name X] [-env E] <file.http>                report per-block validation issues in an .http file")
	fmt.Fprintln(w, "  version                                                print the httpfly version")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "-name restricts either command to the single request declared with \"# @name X\".")
	fmt.Fprintln(w, "-env applies the named environment's variables from httpfly.env.json (next to the")
	fmt.Fprintln(w, "  .http file), overriding the file's own global variables but not a request's local ones.")
	fmt.Fprintln(w, "-s/-silent (run only) prints only response bodies, nothing else -- like curl -s.")
	fmt.Fprintln(w, "-json (run only) prints a JSON array of {name, request, response|error, duration_ms}.")
	fmt.Fprintln(w, "-v/-verbose (run only) also prints TLS connection details (version, cipher, peer certificate).")
	fmt.Fprintln(w, "A redirected request's final URL is always noted, in either output format.")
}
