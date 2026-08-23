package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/cristianradulescu/httpfly/internal/cli"
)

func main() {
	if err := cli.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if !errors.Is(err, cli.ErrSilent) {
			fmt.Fprintln(os.Stderr, "httpfly:", err)
		}
		os.Exit(1)
	}
}
