// Package env loads named variable sets from an http-client.env.json file
// (plus an optional http-client.private.env.json overlay), so the same
// .http file can target different backends (dev, staging, prod, ...)
// without editing it -- select one with "-env <name>".
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
const FileName = "http-client.env.json"

// PrivateFileName is an optional sibling of FileName, in the same
// directory, meant for values that shouldn't be committed (credentials,
// tokens, ...). Its values override FileName's on a per-key basis for any
// environment they both define, and it may also define environments
// FileName doesn't have at all (e.g. a local-only environment with a
// personal API key). Either file may be missing, as long as one exists.
const PrivateFileName = "http-client.private.env.json"

// "$shared" is applied to every environment as a default, overridden by
// that environment's own values on a name conflict. It's dollar-prefixed
// so it can't collide with a real environment someone names "shared".
const sharedKey = "$shared"

// Load reads FileName and PrivateFileName from dir (each optional, but at
// least one must exist) and returns the merged variables for the environment
// named name. Precedence, lowest to highest: FileName's "$shared" <
// FileName's own environment entry < PrivateFileName's "$shared" <
// PrivateFileName's own environment entry -- so a private-file value
// always wins over a public-file one, and each file's own environment
// entry still wins over that file's shared defaults.
func Load(dir, name string) (map[string]string, error) {
	publicEnvs, err := loadFile(filepath.Join(dir, FileName))
	publicMissing := os.IsNotExist(err)
	if err != nil && !publicMissing {
		return nil, err
	}

	privateEnvs, err := loadFile(filepath.Join(dir, PrivateFileName))
	privateMissing := os.IsNotExist(err)
	if err != nil && !privateMissing {
		return nil, err
	}

	if publicMissing && privateMissing {
		return nil, fmt.Errorf("environment %q requested, but neither %s nor %s exists in %s", name, FileName, PrivateFileName, dir)
	}

	publicShared := publicEnvs[sharedKey]
	delete(publicEnvs, sharedKey)
	privateShared := privateEnvs[sharedKey]
	delete(privateEnvs, sharedKey)

	publicVars, hasPublic := publicEnvs[name]
	privateVars, hasPrivate := privateEnvs[name]
	if !hasPublic && !hasPrivate {
		names := make(map[string]bool, len(publicEnvs)+len(privateEnvs))
		for n := range publicEnvs {
			names[n] = true
		}
		for n := range privateEnvs {
			names[n] = true
		}
		available := make([]string, 0, len(names))
		for n := range names {
			available = append(available, n)
		}
		sort.Strings(available)
		return nil, fmt.Errorf("%s: no environment named %q (available: %s)", dir, name, strings.Join(available, ", "))
	}

	merged := make(map[string]string, len(publicShared)+len(publicVars)+len(privateShared)+len(privateVars))
	for _, layer := range []map[string]string{publicShared, publicVars, privateShared, privateVars} {
		for k, v := range layer {
			merged[k] = v
		}
	}
	return merged, nil
}

// loadFile reads and unmarshals one environment file's top-level
// environment map. A missing file returns a nil map and the *os.PathError
// from os.ReadFile (os.IsNotExist(err) is true) so the caller can tell a
// missing file apart from an unreadable or invalid one.
func loadFile(path string) (map[string]map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var envs map[string]map[string]string
	if err := json.Unmarshal(data, &envs); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return envs, nil
}
