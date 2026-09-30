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

func writePrivateEnvFile(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, PrivateFileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadReturnsNamedEnvironment(t *testing.T) {
	dir := t.TempDir()
	writeEnvFile(t, dir, `{
		"dev":  {"host": "http://localhost:8080"},
		"prod": {"host": "https://api.example.com", "api_key": "secret"}
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
		"$shared": {"api_version": "v2", "timeout": "30"},
		"dev": {"host": "http://localhost:8080"}
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
		"$shared": {"timeout": "30"},
		"dev": {"timeout": "5"}
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
		"dev": {"host": "http://localhost:8080"},
		"prod": {"host": "https://api.example.com"}
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
		"$shared": {"timeout": "30"},
		"dev": {"host": "http://localhost:8080"}
	}`)

	_, err := Load(dir, "staging")
	if err == nil {
		t.Fatal("expected an error for an unknown environment name")
	}
	if got := err.Error(); !strings.HasSuffix(got, "(available: dev)") {
		t.Errorf("error = %q, want it to end with \"(available: dev)\" -- \"$shared\" isn't a selectable environment", got)
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

func TestLoadWithoutPrivateFileIsNotAnError(t *testing.T) {
	dir := t.TempDir()
	writeEnvFile(t, dir, `{"dev": {"host": "http://localhost:8080"}}`)

	vars, err := Load(dir, "dev")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := "http://localhost:8080"; vars["host"] != want {
		t.Errorf("host = %q, want %q", vars["host"], want)
	}
}

func TestLoadPrivateOverridesPublicPerKey(t *testing.T) {
	dir := t.TempDir()
	writeEnvFile(t, dir, `{"dev": {"host": "http://localhost:8080", "api_version": "v1"}}`)
	writePrivateEnvFile(t, dir, `{"dev": {"host": "http://localhost:9090"}}`)

	vars, err := Load(dir, "dev")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := map[string]string{"host": "http://localhost:9090", "api_version": "v1"}
	if !reflect.DeepEqual(vars, want) {
		t.Errorf("vars = %+v, want %+v (private host wins, public api_version untouched)", vars, want)
	}
}

func TestLoadPrivateCanDefineAnEnvironmentThePublicFileDoesNotHave(t *testing.T) {
	dir := t.TempDir()
	writeEnvFile(t, dir, `{"dev": {"host": "http://localhost:8080"}}`)
	writePrivateEnvFile(t, dir, `{"local": {"host": "http://localhost:1234", "api_key": "secret"}}`)

	vars, err := Load(dir, "local")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := map[string]string{"host": "http://localhost:1234", "api_key": "secret"}
	if !reflect.DeepEqual(vars, want) {
		t.Errorf("vars = %+v, want %+v", vars, want)
	}
}

func TestLoadPrivateSharedOverridesPublicSharedAndPublicEnv(t *testing.T) {
	dir := t.TempDir()
	writeEnvFile(t, dir, `{
		"$shared": {"timeout": "30", "api_version": "v1"},
		"dev": {"host": "http://localhost:8080"}
	}`)
	writePrivateEnvFile(t, dir, `{"$shared": {"timeout": "5"}}`)

	vars, err := Load(dir, "dev")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := map[string]string{"host": "http://localhost:8080", "api_version": "v1", "timeout": "5"}
	if !reflect.DeepEqual(vars, want) {
		t.Errorf("vars = %+v, want %+v (private $shared should win over public $shared)", vars, want)
	}
}

func TestLoadPrivateEnvStillWinsOverPrivateShared(t *testing.T) {
	dir := t.TempDir()
	writeEnvFile(t, dir, `{"dev": {"host": "http://localhost:8080"}}`)
	writePrivateEnvFile(t, dir, `{
		"$shared": {"timeout": "5"},
		"dev": {"timeout": "1"}
	}`)

	vars, err := Load(dir, "dev")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := "1"; vars["timeout"] != want {
		t.Errorf("timeout = %q, want %q (private env entry should win over private $shared)", vars["timeout"], want)
	}
}

func TestLoadUnknownEnvironmentListsNamesFromBothFiles(t *testing.T) {
	dir := t.TempDir()
	writeEnvFile(t, dir, `{"dev": {"host": "http://localhost:8080"}}`)
	writePrivateEnvFile(t, dir, `{"local": {"host": "http://localhost:1234"}}`)

	_, err := Load(dir, "staging")
	if err == nil {
		t.Fatal("expected an error for an unknown environment name")
	}
	if got := err.Error(); !strings.Contains(got, "dev") || !strings.Contains(got, "local") {
		t.Errorf("error %q should list environment names from both files", got)
	}
}

func TestLoadInvalidPrivateJSON(t *testing.T) {
	dir := t.TempDir()
	writeEnvFile(t, dir, `{"dev": {"host": "http://localhost:8080"}}`)
	writePrivateEnvFile(t, dir, `not json`)

	_, err := Load(dir, "dev")
	if err == nil {
		t.Fatal("expected an error for invalid private JSON")
	}
}
