package parser

import (
	"strings"
	"testing"

	"github.com/cristianradulescu/httpfly/internal/httpfile"
)

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
