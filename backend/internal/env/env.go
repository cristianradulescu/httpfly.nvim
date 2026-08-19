// Package env loads named variable sets from an httpfly.env.json file, so
// the same .http file can target different backends (dev, staging, prod,
// ...) without editing it -- select one with "-env <name>".
package env

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileName is the environment file httpfly looks for, alongside the
// .http file being run.
const FileName = "httpfly.env.json"

// file is the on-disk shape of an environment file:
//
//	{
//	  "shared": { "api_version": "v2" },
//	  "environments": {
//	    "dev":  { "host": "http://localhost:8080" },
//	    "prod": { "host": "https://api.example.com" }
//	  }
//	}
//
// "shared" is optional and applies to every environment as a default,
// overridden by that environment's own values on a name conflict.
type file struct {
	Shared       map[string]string            `json:"shared"`
	Environments map[string]map[string]string `json:"environments"`
}

// Load reads FileName from dir and returns the variables for the
// environment named name, with "shared" merged in as defaults.
func Load(dir, name string) (map[string]string, error) {
	path := filepath.Join(dir, FileName)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("environment %q requested, but %s does not exist in %s", name, FileName, dir)
		}
		return nil, err
	}

	var f file
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	vars, ok := f.Environments[name]
	if !ok {
		names := make([]string, 0, len(f.Environments))
		for n := range f.Environments {
			names = append(names, n)
		}
		sort.Strings(names)
		return nil, fmt.Errorf("%s: no environment named %q (available: %s)", path, name, strings.Join(names, ", "))
	}

	merged := make(map[string]string, len(f.Shared)+len(vars))
	for k, v := range f.Shared {
		merged[k] = v
	}
	for k, v := range vars {
		merged[k] = v
	}
	return merged, nil
}
