package cli

import (
	"fmt"

	"github.com/cristianradulescu/httpfly/internal/httpfile"
)

// selectRequests returns all of requests, or just the one named name if
// name is non-empty. @name is mandatory on every request, so a name
// unambiguously identifies at most one.
func selectRequests(requests []httpfile.Request, name string) ([]httpfile.Request, error) {
	if name == "" {
		return requests, nil
	}
	for _, req := range requests {
		if req.Name == name {
			return []httpfile.Request{req}, nil
		}
	}
	return nil, fmt.Errorf("no request named %q", name)
}
