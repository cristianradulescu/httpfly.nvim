package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianradulescu/httpfly/internal/httpfile"
)

// chdir switches the process's working directory to dir for the duration
// of the test, restoring the original directory afterward -- needed since
// spliceFileReferences resolves relative paths against the process cwd.
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

func TestResolveReResolvesAgainstUpdatedVars(t *testing.T) {
	src := "###\n# @name Get\nGET http://localhost:8080/get?token={{auth_token}} HTTP/1.1\nAuthorization: Bearer {{auth_token}}\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	req := result.Blocks[0].Request

	// At parse time, auth_token was undefined -- placeholder left as-is.
	if want := "http://localhost:8080/get?token={{auth_token}}"; req.URL != want {
		t.Fatalf("parse-time URL = %q, want %q", req.URL, want)
	}

	// Re-resolving the SAME request (its Raw* fields) against updated vars
	// -- as if a pre-request script had just set auth_token -- must pick up
	// the new value, simulating per-request (not just per-parse) resolution.
	resolved, issues := Resolve(req, map[string]string{"auth_token": "abc123"})
	if hasError(issues) {
		t.Fatalf("Resolve issues = %+v, want none", issues)
	}
	if want := "http://localhost:8080/get?token=abc123"; resolved.URL != want {
		t.Errorf("re-resolved URL = %q, want %q", resolved.URL, want)
	}
	if want := "Bearer abc123"; resolved.Headers[0].Value != want {
		t.Errorf("re-resolved header = %q, want %q", resolved.Headers[0].Value, want)
	}
}

func TestResolveLocalVariableStillOverridesWhateverVarsAreGiven(t *testing.T) {
	src := "###\n# @name Get\n@env = local-override\nGET http://localhost:8080/get?env={{env}} HTTP/1.1\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	req := result.Blocks[0].Request
	if want := map[string]string{"env": "local-override"}; req.Variables["env"] != want["env"] {
		t.Fatalf("Variables = %+v, want %+v", req.Variables, want)
	}

	// Simulate the CLI merging a "fresher" global value UNDER the request's
	// own local variable -- local must still win, same as at parse time.
	vars := mergeVars(map[string]string{"env": "from-script"}, req.Variables)
	resolved, _ := Resolve(req, vars)
	if want := "http://localhost:8080/get?env=local-override"; resolved.URL != want {
		t.Errorf("URL = %q, want %q (local variable should win)", resolved.URL, want)
	}
}

func TestResolveOnMalformedRequestLineDoesNotAddSpuriousURLIssue(t *testing.T) {
	// A malformed request line yields RawURL == "" (see parseRequestLine);
	// Resolve must not then also complain the (empty) URL isn't absolute --
	// that would be a redundant, confusing second error for the same root
	// cause.
	req := httpfile.Request{Method: "", RawURL: "", Proto: ""}
	_, issues := Resolve(req, nil)
	if issue := findIssue(issues, "url"); issue != nil {
		t.Errorf("issues = %+v, want no \"url\" issue for an empty RawURL", issues)
	}
}

func TestResolveSplicesWholeBodyFileReference(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/photo.png", []byte("\x89PNG\r\n\x1a\nbinarydata"), 0o644); err != nil {
		t.Fatal(err)
	}
	chdir(t, dir)

	req := httpfile.Request{RawURL: "http://localhost:8080/put", RawBody: "< ./photo.png"}
	resolved, issues := Resolve(req, nil)
	if hasError(issues) {
		t.Fatalf("Resolve issues = %+v, want none", issues)
	}
	if want := "\x89PNG\r\n\x1a\nbinarydata"; resolved.Body != want {
		t.Errorf("Body = %q, want %q (spliced file content)", resolved.Body, want)
	}
}

func TestResolveSplicesFileReferenceWithinMultipartBody(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/avatar.png", []byte("filebytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	chdir(t, dir)

	rawBody := "--B\nContent-Disposition: form-data; name=\"avatar\"; filename=\"avatar.png\"\n\n< ./avatar.png\n--B--"
	req := httpfile.Request{RawURL: "http://localhost:8080/post", RawBody: rawBody}
	resolved, issues := Resolve(req, nil)
	if hasError(issues) {
		t.Fatalf("Resolve issues = %+v, want none", issues)
	}
	want := "--B\nContent-Disposition: form-data; name=\"avatar\"; filename=\"avatar.png\"\n\nfilebytes\n--B--"
	if resolved.Body != want {
		t.Errorf("Body = %q, want %q", resolved.Body, want)
	}
}

func TestResolveFileReferencePathCanUseVariables(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/cert.pem", []byte("cert-content"), 0o644); err != nil {
		t.Fatal(err)
	}
	chdir(t, dir)

	req := httpfile.Request{RawURL: "http://localhost:8080/put", RawBody: "< ./{{filename}}"}
	resolved, issues := Resolve(req, map[string]string{"filename": "cert.pem"})
	if hasError(issues) {
		t.Fatalf("Resolve issues = %+v, want none", issues)
	}
	if want := "cert-content"; resolved.Body != want {
		t.Errorf("Body = %q, want %q", resolved.Body, want)
	}
}

func TestResolveMissingFileReferenceWarns(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	req := httpfile.Request{RawURL: "http://localhost:8080/put", RawBody: "< ./does-not-exist.png"}
	_, issues := Resolve(req, nil)
	issue := findIssue(issues, "body")
	if issue == nil || issue.Severity != SeverityWarning {
		t.Fatalf("issues = %+v, want a body warning for the missing file", issues)
	}
	if !IsMissingFileIssue(*issue) {
		t.Errorf("IsMissingFileIssue(%+v) = false, want true", issue)
	}
}

func TestResolveBodyWithoutFileReferenceIsUnaffected(t *testing.T) {
	req := httpfile.Request{RawURL: "http://localhost:8080/post", RawBody: "{\n  \"name\": \"Ada\"\n}"}
	resolved, issues := Resolve(req, nil)
	if hasError(issues) {
		t.Fatalf("Resolve issues = %+v, want none", issues)
	}
	if want := "{\n  \"name\": \"Ada\"\n}"; resolved.Body != want {
		t.Errorf("Body = %q, want %q (unchanged)", resolved.Body, want)
	}
}

func TestIsMissingFileIssue(t *testing.T) {
	missing := Issue{Element: "body", Severity: SeverityWarning, Message: missingFilePrefix + `"./x.png": open ./x.png: no such file or directory`}
	if !IsMissingFileIssue(missing) {
		t.Errorf("IsMissingFileIssue(%+v) = false, want true", missing)
	}

	other := Issue{Element: "body", Severity: SeverityWarning, Message: "undefined variable \"x\""}
	if IsMissingFileIssue(other) {
		t.Errorf("IsMissingFileIssue(%+v) = true, want false", other)
	}
}

func TestIsUndefinedVariableIssue(t *testing.T) {
	undefined := undefinedVariableIssue("url", "auth_token")
	if !IsUndefinedVariableIssue(undefined) {
		t.Errorf("IsUndefinedVariableIssue(%+v) = false, want true", undefined)
	}

	other := Issue{Element: "url", Severity: SeverityError, Message: "URL must be absolute (missing scheme or host)"}
	if IsUndefinedVariableIssue(other) {
		t.Errorf("IsUndefinedVariableIssue(%+v) = true, want false", other)
	}
}

func TestResolveExpandsVariablesReferencingVariables(t *testing.T) {
	src := "@scheme = http\n@host = {{scheme}}://localhost:8080\n\n###\n# @name Nested\n@base = {{host}}/api\nGET {{base}}/users HTTP/1.1\nX-Origin: {{host}}\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	block := result.Blocks[0]
	if len(block.Issues) != 0 {
		t.Fatalf("issues = %+v, want none", block.Issues)
	}
	if want := "http://localhost:8080/api/users"; block.Request.URL != want {
		t.Errorf("URL = %q, want %q", block.Request.URL, want)
	}
	if want := "http://localhost:8080"; block.Request.Headers[0].Value != want {
		t.Errorf("header = %q, want %q", block.Request.Headers[0].Value, want)
	}
}

func TestResolveVariableCycleIsAnError(t *testing.T) {
	src := "@a = {{b}}\n@b = {{a}}\n\n###\n# @name Cyclic\nGET http://localhost:8080/get?x={{a}} HTTP/1.1\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	issue := findIssue(result.Blocks[0].Issues, "url")
	if issue == nil || issue.Severity != SeverityError || !strings.Contains(issue.Message, "cycle: a -> b -> a") {
		t.Fatalf("issue = %+v, want a cycle error", issue)
	}
	if !result.HasErrors() {
		t.Error("HasErrors() = false, want true")
	}
}

func TestResolveDynamicVariableInURLAndUnsupportedOneWarns(t *testing.T) {
	src := "###\n# @name Dyn\nGET http://localhost:8080/get?id={{$uuid}}&x={{$nope}} HTTP/1.1\nX-Request-Id: {{$uuid}}\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	block := result.Blocks[0]
	if strings.Contains(block.Request.URL, "{{$uuid}}") || !strings.Contains(block.Request.URL, "x={{$nope}}") {
		t.Errorf("URL = %q, want $uuid substituted and $nope left literal", block.Request.URL)
	}
	if strings.Contains(block.Request.Headers[0].Value, "{{") {
		t.Errorf("header = %q, want $uuid substituted", block.Request.Headers[0].Value)
	}
	issue := findIssue(block.Issues, "url")
	if issue == nil || !IsUndefinedVariableIssue(*issue) || !strings.Contains(issue.Message, "$nope") {
		t.Errorf("issue = %+v, want an undefined-variable warning for $nope", issue)
	}
}

func TestResolveMultipartBodyUsesCRLF(t *testing.T) {
	src := strings.Join([]string{
		"###",
		"# @name Multipart",
		"POST http://localhost:8080/post HTTP/1.1",
		"Content-Type: multipart/form-data; boundary=B",
		"",
		"--B",
		"Content-Disposition: form-data; name=\"username\"",
		"",
		"alice",
		"--B--",
	}, "\n")
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	want := "--B\r\nContent-Disposition: form-data; name=\"username\"\r\n\r\nalice\r\n--B--"
	if got := result.Blocks[0].Request.Body; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
	// Idempotent: a file already saved with CRLF line endings comes out the same.
	result, err = Analyze(strings.NewReader(strings.ReplaceAll(src, "\n", "\r\n")))
	if err != nil {
		t.Fatalf("Analyze (CRLF source): %v", err)
	}
	if got := result.Blocks[0].Request.Body; got != want {
		t.Errorf("body from CRLF source = %q, want %q", got, want)
	}
}

func TestResolveNonMultipartBodyKeepsLF(t *testing.T) {
	src := "###\n# @name JSON\nPOST http://localhost:8080/post HTTP/1.1\nContent-Type: application/json\n\n{\n  \"a\": 1\n}\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if got, want := result.Blocks[0].Request.Body, "{\n  \"a\": 1\n}"; got != want {
		t.Errorf("body = %q, want %q (LF untouched outside multipart)", got, want)
	}
}

func TestResolveMultipartCRLFLeavesSplicedFileBytesAlone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.bin")
	fileBytes := "line1\nline2\n\x00\xff"
	if err := os.WriteFile(path, []byte(fileBytes), 0o644); err != nil {
		t.Fatal(err)
	}
	src := strings.Join([]string{
		"###",
		"# @name Upload",
		"POST http://localhost:8080/post HTTP/1.1",
		"Content-Type: multipart/form-data; boundary=B",
		"",
		"--B",
		"Content-Disposition: form-data; name=\"f\"; filename=\"data.bin\"",
		"",
		"< " + path,
		"--B--",
	}, "\n")
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	want := "--B\r\nContent-Disposition: form-data; name=\"f\"; filename=\"data.bin\"\r\n\r\n" + fileBytes + "\r\n--B--"
	if got := result.Blocks[0].Request.Body; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}
