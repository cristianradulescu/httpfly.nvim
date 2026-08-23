package state

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadMissingFileReturnsEmptyMap(t *testing.T) {
	dir := t.TempDir()
	vars, err := Load(dir, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(vars) != 0 {
		t.Errorf("vars = %+v, want empty", vars)
	}
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	dir := t.TempDir()
	want := map[string]string{"auth_token": "abc123"}

	if err := Save(dir, "", want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(dir, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestSaveCreatesStateDir(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, "", map[string]string{"x": "1"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, Dir, FileName)); err != nil {
		t.Errorf("state file not created: %v", err)
	}
}

func TestEnvironmentsAreIsolated(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, "dev", map[string]string{"token": "dev-token"}); err != nil {
		t.Fatalf("Save dev: %v", err)
	}
	if err := Save(dir, "prod", map[string]string{"token": "prod-token"}); err != nil {
		t.Fatalf("Save prod: %v", err)
	}

	dev, err := Load(dir, "dev")
	if err != nil {
		t.Fatalf("Load dev: %v", err)
	}
	if want := "dev-token"; dev["token"] != want {
		t.Errorf("dev token = %q, want %q", dev["token"], want)
	}

	prod, err := Load(dir, "prod")
	if err != nil {
		t.Fatalf("Load prod: %v", err)
	}
	if want := "prod-token"; prod["token"] != want {
		t.Errorf("prod token = %q, want %q", prod["token"], want)
	}
}

func TestSaveDoesNotClobberOtherEnvironments(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, "dev", map[string]string{"a": "1"}); err != nil {
		t.Fatalf("Save dev: %v", err)
	}
	if err := Save(dir, "prod", map[string]string{"b": "2"}); err != nil {
		t.Fatalf("Save prod: %v", err)
	}

	dev, err := Load(dir, "dev")
	if err != nil {
		t.Fatalf("Load dev: %v", err)
	}
	if want := map[string]string{"a": "1"}; !reflect.DeepEqual(dev, want) {
		t.Errorf("dev = %+v, want %+v (unaffected by saving prod)", dev, want)
	}
}

func TestNoEnvBucketIsIsolatedFromNamedEnvironments(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, "", map[string]string{"a": "no-env"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := Save(dir, "dev", map[string]string{"a": "dev"}); err != nil {
		t.Fatalf("Save dev: %v", err)
	}

	noEnv, err := Load(dir, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := "no-env"; noEnv["a"] != want {
		t.Errorf("a = %q, want %q", noEnv["a"], want)
	}
}
