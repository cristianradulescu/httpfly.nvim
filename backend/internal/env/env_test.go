package env

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeEnvFile(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadReturnsNamedEnvironment(t *testing.T) {
	dir := t.TempDir()
	writeEnvFile(t, dir, `{
		"environments": {
			"dev":  {"host": "http://localhost:8080"},
			"prod": {"host": "https://api.example.com", "api_key": "secret"}
		}
	}`)

	vars, err := Load(dir, "prod")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := map[string]string{"host": "https://api.example.com", "api_key": "secret"}
	if !reflect.DeepEqual(vars, want) {
		t.Errorf("vars = %+v, want %+v", vars, want)
	}
}

func TestLoadMergesSharedAsDefaults(t *testing.T) {
	dir := t.TempDir()
	writeEnvFile(t, dir, `{
		"shared": {"api_version": "v2", "timeout": "30"},
		"environments": {
			"dev": {"host": "http://localhost:8080"}
		}
	}`)

	vars, err := Load(dir, "dev")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := map[string]string{"host": "http://localhost:8080", "api_version": "v2", "timeout": "30"}
	if !reflect.DeepEqual(vars, want) {
		t.Errorf("vars = %+v, want %+v", vars, want)
	}
}

func TestLoadEnvironmentOverridesShared(t *testing.T) {
	dir := t.TempDir()
	writeEnvFile(t, dir, `{
		"shared": {"timeout": "30"},
		"environments": {
			"dev": {"timeout": "5"}
		}
	}`)

	vars, err := Load(dir, "dev")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := "5"; vars["timeout"] != want {
		t.Errorf("timeout = %q, want %q (environment should win over shared)", vars["timeout"], want)
	}
}

func TestLoadMissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(dir, "prod")
	if err == nil {
		t.Fatal("expected an error for a missing environment file")
	}
}

func TestLoadUnknownEnvironmentListsAvailable(t *testing.T) {
	dir := t.TempDir()
	writeEnvFile(t, dir, `{
		"environments": {
			"dev": {"host": "http://localhost:8080"},
			"prod": {"host": "https://api.example.com"}
		}
	}`)

	_, err := Load(dir, "staging")
	if err == nil {
		t.Fatal("expected an error for an unknown environment name")
	}
	if got := err.Error(); !strings.Contains(got, "dev") || !strings.Contains(got, "prod") {
		t.Errorf("error %q should list available environments", got)
	}
}

func TestLoadUnknownEnvironmentDoesNotListSharedAsAnEnvironment(t *testing.T) {
	dir := t.TempDir()
	writeEnvFile(t, dir, `{
		"shared": {"timeout": "30"},
		"environments": {
			"dev": {"host": "http://localhost:8080"}
		}
	}`)

	_, err := Load(dir, "staging")
	if err == nil {
		t.Fatal("expected an error for an unknown environment name")
	}
	if got := err.Error(); !strings.HasSuffix(got, "(available: dev)") {
		t.Errorf("error = %q, want it to end with \"(available: dev)\" -- \"shared\" isn't a selectable environment", got)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	writeEnvFile(t, dir, `not json`)

	_, err := Load(dir, "dev")
	if err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
}
