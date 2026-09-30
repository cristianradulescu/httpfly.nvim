package cli

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianradulescu/httpfly/internal/env"
)

// chdir switches the process's working directory to dir for the duration
// of the test, restoring the original directory afterward -- needed since
// configDir (and everything built on it) reads os.Getwd().
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Fatal(err)
		}
	})
}

func writeEnvFile(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, env.FileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveEnvVarsUsesCurrentWorkingDirectory(t *testing.T) {
	parent := t.TempDir()
	writeEnvFile(t, parent, `{"dev": {"host": "http://parent-dir"}}`)
	chdir(t, parent)

	vars, err := resolveEnvVars("dev")
	if err != nil {
		t.Fatalf("resolveEnvVars: %v", err)
	}
	if want := "http://parent-dir"; vars["host"] != want {
		t.Errorf("host = %q, want %q", vars["host"], want)
	}
}

func TestRunCreatesStateInCurrentWorkingDirectoryNotHTTPFilesDirectory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	parent := t.TempDir()
	v1 := filepath.Join(parent, "v1")
	if err := os.Mkdir(v1, 0o755); err != nil {
		t.Fatal(err)
	}
	httpPath := filepath.Join(v1, "login.http")
	content := "###\n# @name Login\nGET " + srv.URL + " HTTP/1.1\n\n> {%\n  client.global:set(\"x\", \"1\")\n%}\n"
	if err := os.WriteFile(httpPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	chdir(t, parent)

	var stdout bytes.Buffer
	if err := runCommand([]string{filepath.Join("v1", "login.http")}, &stdout, io.Discard); err != nil {
		t.Fatalf("runCommand: %v", err)
	}

	if _, err := os.Stat(filepath.Join(parent, ".httpfly", "state.json")); err != nil {
		t.Errorf("state.json not created in the parent (CWD) directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(v1, ".httpfly")); err == nil {
		t.Error(".httpfly was created next to the .http file, want it only in the CWD")
	}
}

func TestResolveEnvVarsIgnoresHTTPFilesOwnDirectory(t *testing.T) {
	parent := t.TempDir()
	writeEnvFile(t, parent, `{"dev": {"host": "http://parent-dir"}}`)

	// An env file physically next to the .http file must NOT be picked up
	// -- only the current working directory's counts.
	sub := filepath.Join(parent, "v1")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeEnvFile(t, sub, `{"dev": {"host": "http://sibling-dir"}}`)

	chdir(t, parent)

	vars, err := resolveEnvVars("dev")
	if err != nil {
		t.Fatalf("resolveEnvVars: %v", err)
	}
	if want := "http://parent-dir"; vars["host"] != want {
		t.Errorf("host = %q, want the CWD's env file value %q, not the .http file directory's", vars["host"], want)
	}
}
