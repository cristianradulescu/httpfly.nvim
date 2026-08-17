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
	fmt.Fprintln(w, "  run [-name X] <file.http>        send the requests in an .http file and print their responses")
	fmt.Fprintln(w, "  validate [-name X] <file.http>   report per-block validation issues in an .http file")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "-name restricts either command to the single request declared with \"# @name X\".")
}
