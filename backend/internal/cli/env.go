package cli

import (
	"os"

	"github.com/cristianradulescu/httpfly/internal/env"
)

// configDir returns the directory httpfly looks in for http-client.env.json
// and creates/reads .httpfly/ in: the current working directory (where
// the httpfly binary was launched), not the directory containing the
// .http file being acted on. This lets several .http files in different
// subdirectories (e.g. v1/, v2/ of an API) share one environment file and
// one persisted client.global state, as long as httpfly is invoked from
// their common parent directory.
func configDir() (string, error) {
	return os.Getwd()
}

// resolveEnvVars returns the variables for the named environment, looked
// up in configDir's http-client.env.json, or nil if name is empty (no -env
// flag given).
func resolveEnvVars(name string) (map[string]string, error) {
	if name == "" {
		return nil, nil
	}
	dir, err := configDir()
	if err != nil {
		return nil, err
	}
	return env.Load(dir, name)
}

// mergeVars layers override's entries over base's, without mutating either.
func mergeVars(base, override map[string]string) map[string]string {
	merged := make(map[string]string, len(base)+len(override))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range override {
		merged[k] = v
	}
	return merged
}
