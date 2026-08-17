// Package parser turns .http file content into httpfile.File values.
package parser

import (
	"fmt"
	"io"

	"github.com/cristianradulescu/httpfly/internal/httpfile"
)

// Parse reads r and returns the requests it defines.
func Parse(r io.Reader) (*httpfile.File, error) {
	return nil, fmt.Errorf("parser: not implemented yet")
}
