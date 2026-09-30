package cli

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/cristianradulescu/httpfly/internal/curl"
	"github.com/cristianradulescu/httpfly/internal/parser"
	"github.com/cristianradulescu/httpfly/internal/state"
)

// defaultConvertedRequestName is the @name a converted request gets when
// -name isn't given -- httpfly requires a non-empty @name on every
// request, so something has to fill in for the name curl commands don't
// carry.
const defaultConvertedRequestName = "ConvertedRequest"

func convertCommand(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("convert: expected a source, e.g. \"convert from-curl\"")
	}
	source := args[0]
	switch source {
	case "from-curl":
		return convertFromCurlCommand(args[1:], stdin, stdout, stderr)
	case "to-curl":
		return convertToCurlCommand(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("convert: unknown source %q (supported: from-curl, to-curl)", source)
	}
}

func convertFromCurlCommand(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("convert from-curl", flag.ContinueOnError)
	fs.SetOutput(stderr)
	name := fs.String("name", "", "name for the generated request (@name); defaults to \""+defaultConvertedRequestName+"\"")
	file := fs.String("file", "", "read the curl command from this file instead of stdin")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("convert from-curl: unexpected argument %q (pipe the curl command via stdin, or use -file)", fs.Arg(0))
	}

	var input []byte
	var err error
	if *file != "" {
		input, err = os.ReadFile(*file)
	} else {
		input, err = io.ReadAll(stdin)
	}
	if err != nil {
		return fmt.Errorf("convert from-curl: %w", err)
	}

	req, warnings, err := curl.Parse(string(input))
	if err != nil {
		return fmt.Errorf("convert from-curl: %w", err)
	}

	req.Name = *name
	if req.Name == "" {
		req.Name = defaultConvertedRequestName
	}
	req.Proto = "HTTP/1.1"

	for _, w := range warnings {
		fmt.Fprintln(stderr, "convert from-curl:", w)
	}

	fmt.Fprint(stdout, curl.Format(req))
	return nil
}

// convertToCurlCommand converts one request from an .http file into a
// multi-line, bash-style curl command. It never runs scripts (same
// read-only principle as "validate") -- a request whose {{var}} placeholder
// is still undefined after resolving against the prelude/-env/persisted
// client.global state is a hard error here, since a curl command
// containing a literal "{{name}}" would just be broken, with no later
// chance to fill it in the way "run" has.
func convertToCurlCommand(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("convert to-curl", flag.ContinueOnError)
	fs.SetOutput(stderr)
	name := fs.String("name", "", "only convert the request with this @name (required if the file has more than one request)")
	envName := fs.String("env", "", "apply variables from the named environment in http-client.env.json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("convert to-curl: expected exactly one .http file argument")
	}
	path := fs.Arg(0)

	dir, err := configDir()
	if err != nil {
		return fmt.Errorf("convert to-curl: %w", err)
	}

	envVars, err := resolveEnvVars(*envName)
	if err != nil {
		return fmt.Errorf("convert to-curl: %w", err)
	}
	persisted, err := state.Load(dir, *envName)
	if err != nil {
		return fmt.Errorf("convert to-curl: %w", err)
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	result, err := parser.AnalyzeWithEnv(f, mergeVars(envVars, persisted))
	if err != nil {
		return err
	}

	if *name == "" && len(result.Blocks) > 1 {
		return fmt.Errorf("convert to-curl: %s has %d requests; use -name to pick one", path, len(result.Blocks))
	}
	if *name != "" {
		result, err = filterResult(result, *name)
		if err != nil {
			return fmt.Errorf("convert to-curl: %w", err)
		}
	}
	if len(result.Blocks) == 0 {
		return fmt.Errorf("convert to-curl: %s has no requests", path)
	}

	block := result.Blocks[0]
	if err := issuesAsFatal(block.Issues, "convert to-curl"); err != nil {
		return err
	}

	fmt.Fprint(stdout, curl.ToCurl(block.Request))
	return nil
}
