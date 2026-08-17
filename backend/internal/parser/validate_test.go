package parser

import (
	"reflect"
	"strings"
	"testing"
)

func findIssue(issues []Issue, element string) *Issue {
	for i := range issues {
		if issues[i].Element == element {
			return &issues[i]
		}
	}
	return nil
}

func TestAnalyzeMissingNameErrors(t *testing.T) {
	src := "###\nGET http://localhost:8080/get HTTP/1.1\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	issue := findIssue(result.Blocks[0].Issues, "metadata:name")
	if issue == nil || issue.Severity != SeverityError {
		t.Fatalf("issue = %+v, want an error for missing @name", issue)
	}
	if !result.HasErrors() {
		t.Errorf("HasErrors() = false, want true")
	}
}

func TestAnalyzeEmptyNameDoesNotAlsoErrorAsMissing(t *testing.T) {
	src := "###\n# @name\nGET http://localhost:8080/get HTTP/1.1\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	var nameIssues int
	for _, issue := range result.Blocks[0].Issues {
		if issue.Element == "metadata:name" {
			nameIssues++
		}
	}
	if nameIssues != 1 {
		t.Errorf("got %d metadata:name issues, want exactly 1: %+v", nameIssues, result.Blocks[0].Issues)
	}
}

func TestAnalyzeBasicExampleHasNoIssues(t *testing.T) {
	src := "###\n# @name Get\nGET http://localhost:8080/get?greeting=hello HTTP/1.1\nAccept: application/json\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(result.Blocks) != 1 {
		t.Fatalf("got %d blocks, want 1", len(result.Blocks))
	}
	if got := result.Blocks[0].Issues; len(got) != 0 {
		t.Errorf("Issues = %+v, want none", got)
	}
	if result.HasErrors() {
		t.Errorf("HasErrors() = true, want false")
	}
}

func TestAnalyzeUnknownMetadataWarns(t *testing.T) {
	src := "###\n# @timeout 30\nGET http://localhost:8080/get HTTP/1.1\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	issue := findIssue(result.Blocks[0].Issues, "metadata:timeout")
	if issue == nil {
		t.Fatalf("no issue for unknown metadata, got %+v", result.Blocks[0].Issues)
	}
	if issue.Severity != SeverityWarning {
		t.Errorf("Severity = %v, want Warning", issue.Severity)
	}
}

func TestAnalyzeLangMetadataWarnsUnsupported(t *testing.T) {
	src := "###\n# @lang javascript\nGET http://localhost:8080/get HTTP/1.1\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	issue := findIssue(result.Blocks[0].Issues, "metadata:lang")
	if issue == nil || issue.Severity != SeverityWarning {
		t.Fatalf("issue = %+v, want a warning", issue)
	}
}

func TestAnalyzeNonStandardMethodWarns(t *testing.T) {
	src := "###\nFETCH http://localhost:8080/get HTTP/1.1\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	issue := findIssue(result.Blocks[0].Issues, "method")
	if issue == nil || issue.Severity != SeverityWarning {
		t.Fatalf("issue = %+v, want a warning", issue)
	}
}

func TestAnalyzeRelativeURLErrors(t *testing.T) {
	src := "###\nGET /get HTTP/1.1\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	issue := findIssue(result.Blocks[0].Issues, "url")
	if issue == nil || issue.Severity != SeverityError {
		t.Fatalf("issue = %+v, want an error", issue)
	}
	if !result.HasErrors() {
		t.Errorf("HasErrors() = false, want true")
	}
}

func TestAnalyzeUndefinedVariableWarns(t *testing.T) {
	src := "###\nGET http://localhost:8080/get?id={{request_id}} HTTP/1.1\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	issue := findIssue(result.Blocks[0].Issues, "url")
	if issue == nil || issue.Severity != SeverityWarning {
		t.Fatalf("issue = %+v, want a warning", issue)
	}
	if got := result.Blocks[0].Request.URL; got != "http://localhost:8080/get?id={{request_id}}" {
		t.Errorf("URL = %q, want placeholder left untouched", got)
	}
}

func TestAnalyzeResolvesFileScopedVariables(t *testing.T) {
	src := "@host = http://localhost:8080\n@greeting = hello\n\n###\n# @name Get\nGET {{host}}/get?greeting={{greeting}} HTTP/1.1\nAuthorization: Bearer {{host}}\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.HasErrors() {
		t.Fatalf("HasErrors() = true, want false: %+v", result.Blocks)
	}
	if want, got := map[string]string{"host": "http://localhost:8080", "greeting": "hello"}, result.Variables; !reflect.DeepEqual(want, got) {
		t.Errorf("Variables = %+v, want %+v", got, want)
	}
	req := result.Blocks[0].Request
	if want := "http://localhost:8080/get?greeting=hello"; req.URL != want {
		t.Errorf("URL = %q, want %q", req.URL, want)
	}
	if want := "Bearer http://localhost:8080"; req.Headers[0].Value != want {
		t.Errorf("Header value = %q, want %q", req.Headers[0].Value, want)
	}
}

func TestAnalyzeHeaderWithUndefinedVariableIsStillKept(t *testing.T) {
	src := "###\nGET http://localhost:8080/get HTTP/1.1\nAuthorization: Bearer {{token}}\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	headers := result.Blocks[0].Request.Headers
	if len(headers) != 1 || headers[0].Name != "Authorization" {
		t.Fatalf("Headers = %+v, want Authorization to still be present despite the warning", headers)
	}
	if want := "Bearer {{token}}"; headers[0].Value != want {
		t.Errorf("Header value = %q, want %q", headers[0].Value, want)
	}
}

func TestAnalyzeUndefinedVariableInHeaderAndBodyWarns(t *testing.T) {
	src := "###\nPOST http://localhost:8080/post HTTP/1.1\nAuthorization: Bearer {{token}}\n\n{\"id\": \"{{request_id}}\"}\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if findIssue(result.Blocks[0].Issues, "header:Authorization") == nil {
		t.Errorf("no issue for undefined header variable, got %+v", result.Blocks[0].Issues)
	}
	if findIssue(result.Blocks[0].Issues, "body") == nil {
		t.Errorf("no issue for undefined body variable, got %+v", result.Blocks[0].Issues)
	}
}

func TestAnalyzeUnrecognizedProtoWarns(t *testing.T) {
	src := "###\nGET http://localhost:8080/get WEIRD/9\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	issue := findIssue(result.Blocks[0].Issues, "proto")
	if issue == nil || issue.Severity != SeverityWarning {
		t.Fatalf("issue = %+v, want a warning", issue)
	}
}

func TestAnalyzeScriptingBlockErrors(t *testing.T) {
	src := "###\n< {%\n  request.variables.set(\"id\", 1);\n%}\n\nGET http://localhost:8080/get HTTP/1.1\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !result.HasErrors() {
		t.Fatalf("HasErrors() = false, want true, blocks: %+v", result.Blocks)
	}
}

func TestParseSurfacesAggregatedErrors(t *testing.T) {
	src := "###\nGET /get\n###\nPOST /post\n"
	_, err := Parse(strings.NewReader(src))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "request 1") || !strings.Contains(err.Error(), "request 2") {
		t.Errorf("error %q should mention both requests", err.Error())
	}
}
