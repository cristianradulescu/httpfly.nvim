package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunDownloadSavesBodyToFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte{0xFF, 0xD8, 0xFF, 0x00, 0x01, 0x02}) // arbitrary "binary" bytes
	}))
	defer srv.Close()

	dir := t.TempDir()
	out := filepath.Join(dir, "photo.jpg")
	path := writeHTTPFile(t, "###\n# @name Get\nGET "+srv.URL+" HTTP/1.1\n")

	var stdout bytes.Buffer
	if err := runCommand([]string{"-download", out, path}, &stdout); err != nil {
		t.Fatalf("runCommand: %v", err)
	}

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reading downloaded file: %v", err)
	}
	want := []byte{0xFF, 0xD8, 0xFF, 0x00, 0x01, 0x02}
	if !bytes.Equal(got, want) {
		t.Errorf("downloaded file content = %v, want %v", got, want)
	}

	stdoutStr := stdout.String()
	if !strings.Contains(stdoutStr, "[saved 6 bytes to "+out+"]") {
		t.Errorf("stdout = %q, want a \"[saved 6 bytes to ...]\" confirmation", stdoutStr)
	}
	if strings.Contains(stdoutStr, "\xFF\xD8") {
		t.Errorf("stdout = %q, raw body bytes leaked into terminal output", stdoutStr)
	}
}

func TestRunDownloadJSONModeEmptiesBodyAndAddsDownloadPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	out := filepath.Join(dir, "body.bin")
	path := writeHTTPFile(t, "###\n# @name Get\nGET "+srv.URL+" HTTP/1.1\n")

	var stdout bytes.Buffer
	if err := runCommand([]string{"-download", out, "-json", path}, &stdout); err != nil {
		t.Fatalf("runCommand: %v", err)
	}

	jsonOut := stdout.String()
	if strings.Contains(jsonOut, `"body": "hello"`) {
		t.Errorf("JSON output = %s, want body emptied when downloading", jsonOut)
	}
	if !strings.Contains(jsonOut, `"download_path": "`+out+`"`) {
		t.Errorf("JSON output = %s, want a download_path field with %q", jsonOut, out)
	}
}

func TestRunDownloadRequiresSingleRequest(t *testing.T) {
	path := writeHTTPFile(t, "###\n# @name A\nGET http://localhost:8080/a HTTP/1.1\n\n###\n# @name B\nGET http://localhost:8080/b HTTP/1.1\n")
	dir := t.TempDir()

	var stdout bytes.Buffer
	err := runCommand([]string{"-download", filepath.Join(dir, "out.bin"), path}, &stdout)
	if err == nil {
		t.Fatal("expected an error when -download is used with more than one selected request")
	}
}

func TestRunDownloadAndSilentAreMutuallyExclusive(t *testing.T) {
	path := writeHTTPFile(t, "###\n# @name Get\nGET http://localhost:8080/get HTTP/1.1\n")
	dir := t.TempDir()

	var stdout bytes.Buffer
	err := runCommand([]string{"-download", filepath.Join(dir, "out.bin"), "-s", path}, &stdout)
	if err == nil {
		t.Fatal("expected an error when -download and -silent are combined")
	}
}

func TestRunDownloadFailsClearlyWhenParentDirMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("data"))
	}))
	defer srv.Close()

	path := writeHTTPFile(t, "###\n# @name Get\nGET "+srv.URL+" HTTP/1.1\n")
	missingDir := filepath.Join(t.TempDir(), "does-not-exist", "out.bin")

	var stdout bytes.Buffer
	err := runCommand([]string{"-download", missingDir, path}, &stdout)
	if err == nil {
		t.Fatal("expected an error when the download path's parent directory doesn't exist")
	}
}

func TestRunDownloadOverwritesExistingFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("new-content"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	out := filepath.Join(dir, "out.bin")
	if err := os.WriteFile(out, []byte("old-content-that-is-longer"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := writeHTTPFile(t, "###\n# @name Get\nGET "+srv.URL+" HTTP/1.1\n")

	var stdout bytes.Buffer
	if err := runCommand([]string{"-download", out, path}, &stdout); err != nil {
		t.Fatalf("runCommand: %v", err)
	}

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-content" {
		t.Errorf("file content = %q, want it fully overwritten to %q", got, "new-content")
	}
}

func TestRunDownloadNotWrittenOnTransportFailure(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out.bin")
	path := writeHTTPFile(t, "###\n# @name Fails\nGET http://127.0.0.1:1/get HTTP/1.1\n")

	var stdout bytes.Buffer
	err := runCommand([]string{"-download", out, path}, &stdout)
	if err == nil {
		t.Fatal("expected an error since the request itself failed to send")
	}
	if _, statErr := os.Stat(out); statErr == nil {
		t.Error("download file was created despite the request failing to send")
	}
}
