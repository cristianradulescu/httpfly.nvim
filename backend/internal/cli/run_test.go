package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeHTTPFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.http")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunJSONModeSuppressesFailureSummary(t *testing.T) {
	// 127.0.0.1:1 is a guaranteed, immediate connection refusal -- no real
	// network access needed for a deterministic transport failure.
	path := writeHTTPFile(t, "###\n# @name Fails\nGET http://127.0.0.1:1/get HTTP/1.1\n")

	var stdout bytes.Buffer
	err := runCommand([]string{"-json", path}, &stdout)

	if err == nil {
		t.Fatal("expected a non-nil error (for the exit code) when a request fails")
	}
	if !errors.Is(err, ErrSilent) {
		t.Errorf("err = %v, want it to be (or wrap) ErrSilent so main.go prints nothing extra", err)
	}
	if err.Error() != "" {
		t.Errorf("err.Error() = %q, want empty -- printing it would add a non-JSON artifact after the array", err.Error())
	}

	out := stdout.String()
	if !strings.HasSuffix(out, "]\n") {
		t.Errorf("stdout doesn't end with the JSON array's closing \"]\\n\" -- something was printed after it: %q", out)
	}
}

func TestRunPlainModeStillReportsFailureSummary(t *testing.T) {
	path := writeHTTPFile(t, "###\n# @name Fails\nGET http://127.0.0.1:1/get HTTP/1.1\n")

	var stdout bytes.Buffer
	err := runCommand([]string{path}, &stdout)

	if err == nil {
		t.Fatal("expected an error")
	}
	if errors.Is(err, ErrSilent) {
		t.Error("plain-text mode should still report a human-readable failure summary, not ErrSilent")
	}
	if err.Error() == "" {
		t.Error("plain-text mode's error message should not be empty")
	}
}
