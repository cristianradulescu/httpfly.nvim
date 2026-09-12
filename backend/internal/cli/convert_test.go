package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestConvertFromCurlToHTTP(t *testing.T) {
	stdin := strings.NewReader(`curl 'https://example.com/get' -H 'Accept: application/json'`)
	var stdout, stderr bytes.Buffer

	err := convertCommand([]string{"from-curl", "-name", "GetExample"}, stdin, &stdout, &stderr)
	if err != nil {
		t.Fatalf("convertCommand: %v", err)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
	if !strings.Contains(stdout.String(), "# @name GetExample") {
		t.Errorf("stdout = %q, want it to contain \"# @name GetExample\"", stdout.String())
	}
}

func TestConvertFromCurlWarningsGoToStderrNotStdout(t *testing.T) {
	stdin := strings.NewReader(`curl 'https://example.com' -k`)
	var stdout, stderr bytes.Buffer

	if err := convertCommand([]string{"from-curl"}, stdin, &stdout, &stderr); err != nil {
		t.Fatalf("convertCommand: %v", err)
	}
	if strings.Contains(stdout.String(), "insecure") {
		t.Errorf("stdout = %q, warning text leaked into the generated .http output", stdout.String())
	}
	if !strings.Contains(stderr.String(), "insecure") {
		t.Errorf("stderr = %q, want it to mention the dropped -k flag", stderr.String())
	}
}

func TestConvertToCurlBasic(t *testing.T) {
	path := writeHTTPFile(t, "###\n# @name Get\nGET http://localhost:8080/get?greeting=hello HTTP/1.1\nAccept: application/json\n")

	var stdout, stderr bytes.Buffer
	err := convertCommand([]string{"to-curl", path}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("convertCommand: %v", err)
	}

	out := stdout.String()
	if !strings.HasPrefix(out, "curl 'http://localhost:8080/get?greeting=hello'") {
		t.Errorf("stdout = %q, want it to start with the curl URL", out)
	}
	if !strings.Contains(out, "-H 'Accept: application/json'") {
		t.Errorf("stdout = %q, want the Accept header", out)
	}
}

func TestConvertToCurlRequiresNameWhenMultipleRequests(t *testing.T) {
	path := writeHTTPFile(t, "###\n# @name Get\nGET http://localhost:8080/get HTTP/1.1\n\n###\n# @name Post\nPOST http://localhost:8080/post HTTP/1.1\n")

	var stdout, stderr bytes.Buffer
	err := convertCommand([]string{"to-curl", path}, nil, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected an error when the file has multiple requests and -name is omitted")
	}
}

func TestConvertToCurlSelectsNamedRequest(t *testing.T) {
	path := writeHTTPFile(t, "###\n# @name Get\nGET http://localhost:8080/get HTTP/1.1\n\n###\n# @name Post\nPOST http://localhost:8080/post HTTP/1.1\n")

	var stdout, stderr bytes.Buffer
	err := convertCommand([]string{"to-curl", "-name", "Post", path}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("convertCommand: %v", err)
	}
	if !strings.Contains(stdout.String(), "http://localhost:8080/post") {
		t.Errorf("stdout = %q, want the Post request's URL", stdout.String())
	}
	if !strings.Contains(stdout.String(), "-X 'POST'") {
		t.Errorf("stdout = %q, want -X 'POST'", stdout.String())
	}
}

func TestConvertToCurlUndefinedVariableIsFatal(t *testing.T) {
	path := writeHTTPFile(t, "###\n# @name Get\nGET http://localhost:8080/get?token={{missing}} HTTP/1.1\n")

	var stdout, stderr bytes.Buffer
	err := convertCommand([]string{"to-curl", path}, nil, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected an error for a still-undefined variable")
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want nothing written when conversion fails", stdout.String())
	}
}

func TestConvertToCurlMissingFileReferenceIsFatal(t *testing.T) {
	path := writeHTTPFile(t, "###\n# @name Put\nPUT http://localhost:8080/put HTTP/1.1\n\n< ./does-not-exist.png\n")

	var stdout, stderr bytes.Buffer
	err := convertCommand([]string{"to-curl", path}, nil, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected an error for a body file reference that can't be read")
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want nothing written when conversion fails", stdout.String())
	}
}
