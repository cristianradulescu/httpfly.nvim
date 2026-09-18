package cli

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	err := runCommand([]string{"-json", path}, &stdout, io.Discard)

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
	err := runCommand([]string{path}, &stdout, io.Discard)

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

func TestRunFlagErrorsGoToStderrNotStdout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := runCommand([]string{"-json", "-bogus", "whatever.http"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("runCommand: want an error for an unknown flag")
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty (flag errors/usage must not pollute -json output)", stdout.String())
	}
	if !strings.Contains(stderr.String(), "-bogus") {
		t.Errorf("stderr = %q, want the flag error", stderr.String())
	}
}

func TestRunPreScriptErrorReportedOnce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pre.http")
	src := "###\n# @name PreErr\n< {%\n  error(\"boom\")\n%}\nGET http://127.0.0.1:9/get HTTP/1.1\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	if err := runCommand([]string{"-json", path}, &stdout, io.Discard); err == nil {
		t.Fatal("runCommand: want a failure exit")
	}
	if got := strings.Count(stdout.String(), "pre-request script:"); got != 1 {
		t.Errorf("prefix appears %d times in output, want exactly 1:\n%s", got, stdout.String())
	}
}

func TestRunTimeoutFlag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.Write([]byte("late"))
	}))
	defer srv.Close()
	path := writeHTTPFile(t, "###\n# @name Slow\nGET "+srv.URL+"/ HTTP/1.1\n")

	var stdout bytes.Buffer
	if err := runCommand([]string{"-timeout", "50ms", path}, &stdout, io.Discard); err == nil {
		t.Fatalf("want a failure with -timeout 50ms against a 300ms server; output:\n%s", stdout.String())
	} else if !strings.Contains(stdout.String(), "Timeout exceeded") && !strings.Contains(stdout.String(), "deadline") {
		t.Errorf("output doesn't mention the timeout:\n%s", stdout.String())
	}

	stdout.Reset()
	if err := runCommand([]string{"-timeout", "5s", path}, &stdout, io.Discard); err != nil {
		t.Fatalf("with -timeout 5s: %v\n%s", err, stdout.String())
	}
	if !strings.Contains(stdout.String(), "late") {
		t.Errorf("output missing the response body:\n%s", stdout.String())
	}

	if err := runCommand([]string{"-timeout", "-1s", path}, &stdout, io.Discard); err == nil || !strings.Contains(err.Error(), "-timeout") {
		t.Errorf("negative -timeout: err = %v, want a rejection", err)
	}
}
