// Package state persists variables scripts set via "client.global" across
// separate "httpfly run" invocations, so e.g. an auth token fetched by one
// run is still there the next time you run a different request that needs
// it.
package state

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Dir is the directory httpfly stores its state in, alongside the .http
// file being run.
const Dir = ".httpfly"

// FileName is the state file within Dir.
const FileName = "state.json"

// Load returns the persisted global variables for the given directory and
// environment name (pass "" if -env wasn't used; environments are kept in
// separate buckets so a value set while running against "prod" never leaks
// into a "dev" run). A missing state file, or a file with no bucket for
// this environment yet, is normal -- nothing has been persisted yet -- and
// returns an empty map, not an error.
func Load(dir, envName string) (map[string]string, error) {
	buckets, err := readBuckets(dir)
	if err != nil {
		return nil, err
	}
	vars := buckets[envName]
	if vars == nil {
		vars = make(map[string]string)
	}
	return vars, nil
}

// Save writes vars as the persisted global variables for the given
// directory and environment name, read-modify-writing the state file so
// other environments' buckets are left untouched. It creates Dir if it
// doesn't exist yet.
func Save(dir, envName string, vars map[string]string) error {
	buckets, err := readBuckets(dir)
	if err != nil {
		return err
	}
	if buckets == nil {
		buckets = make(map[string]map[string]string)
	}
	buckets[envName] = vars

	stateDir := filepath.Join(dir, Dir)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(buckets, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(stateDir, FileName), data, 0o600)
}

func readBuckets(dir string) (map[string]map[string]string, error) {
	path := filepath.Join(dir, Dir, FileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var buckets map[string]map[string]string
	if err := json.Unmarshal(data, &buckets); err != nil {
		return nil, err
	}
	return buckets, nil
}
