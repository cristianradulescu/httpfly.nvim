package parser

import (
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

func TestAnalyzeVariablePlaceholderWarns(t *testing.T) {
	src := "###\nGET http://localhost:8080/get?id={{request_id}} HTTP/1.1\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	issue := findIssue(result.Blocks[0].Issues, "url")
	if issue == nil || issue.Severity != SeverityWarning {
		t.Fatalf("issue = %+v, want a warning", issue)
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
