package cli

import (
	"path/filepath"

	"github.com/cristianradulescu/httpfly/internal/env"
)

// resolveEnvVars returns the variables for the named environment, looked up
// in the httpfly.env.json alongside the .http file at path, or nil if name
// is empty (no -env flag given).
func resolveEnvVars(path, name string) (map[string]string, error) {
	if name == "" {
		return nil, nil
	}
	return env.Load(filepath.Dir(path), name)
}
