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

func TestAnalyzeNameFromSeparatorLine(t *testing.T) {
	src := "### GetUsers\nGET http://localhost:8080/get HTTP/1.1\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.HasErrors() {
		t.Fatalf("HasErrors() = true, want false: %+v", result.Blocks[0].Issues)
	}
	if got, want := result.Blocks[0].Request.Name, "GetUsers"; got != want {
		t.Errorf("Name = %q, want %q", got, want)
	}
}

func TestAnalyzeNameFromSeparatorLineMatchingExplicitNameIsFine(t *testing.T) {
	src := "### GetUsers\n# @name GetUsers\nGET http://localhost:8080/get HTTP/1.1\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.HasErrors() {
		t.Fatalf("HasErrors() = true, want false: %+v", result.Blocks[0].Issues)
	}
	if got, want := result.Blocks[0].Request.Name, "GetUsers"; got != want {
		t.Errorf("Name = %q, want %q", got, want)
	}
}

func TestAnalyzeNameConflictBetweenSeparatorAndMetadataErrors(t *testing.T) {
	src := "### Foo\n# @name Bar\nGET http://localhost:8080/get HTTP/1.1\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	issue := findIssue(result.Blocks[0].Issues, "metadata:name")
	if issue == nil || issue.Severity != SeverityError {
		t.Fatalf("issue = %+v, want an error for conflicting names", issue)
	}
	if !result.HasErrors() {
		t.Errorf("HasErrors() = false, want true")
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

func TestAnalyzePreRequestScriptCaptured(t *testing.T) {
	src := "###\n# @name Get\n< {%\n  request.variables:set(\"id\", 1)\n%}\n\nGET http://localhost:8080/get?id={{id}} HTTP/1.1\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.HasErrors() {
		t.Fatalf("HasErrors() = true, want false: %+v", result.Blocks[0].Issues)
	}
	if want, got := "  request.variables:set(\"id\", 1)", result.Blocks[0].Request.PreScript; got != want {
		t.Errorf("PreScript = %q, want %q", got, want)
	}
	if result.Blocks[0].Request.Lang != "lua" {
		t.Errorf("Lang = %q, want %q", result.Blocks[0].Request.Lang, "lua")
	}
}

func TestAnalyzePostRequestScriptCapturedAndSeparatedFromBody(t *testing.T) {
	src := "###\n# @name Post\nPOST http://localhost:8080/post HTTP/1.1\n\n{\"a\": 1}\n\n> {%\n  client.global:set(\"x\", 1)\n%}\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.HasErrors() {
		t.Fatalf("HasErrors() = true, want false: %+v", result.Blocks[0].Issues)
	}
	req := result.Blocks[0].Request
	if want := "{\"a\": 1}"; req.Body != want {
		t.Errorf("Body = %q, want %q", req.Body, want)
	}
	if want := "  client.global:set(\"x\", 1)"; req.PostScript != want {
		t.Errorf("PostScript = %q, want %q", req.PostScript, want)
	}
}

func TestAnalyzeUnterminatedPreScriptErrors(t *testing.T) {
	src := "###\n# @name Get\n< {%\n  request.variables:set(\"id\", 1)\nGET http://localhost:8080/get HTTP/1.1\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	issue := findIssue(result.Blocks[0].Issues, "script:pre")
	if issue == nil || issue.Severity != SeverityError {
		t.Fatalf("issue = %+v, want an error for an unterminated script block", issue)
	}
}

func TestAnalyzeMalformedScriptOpenErrors(t *testing.T) {
	src := "###\n# @name Get\n<not-a-script-open\nGET http://localhost:8080/get HTTP/1.1\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	issue := findIssue(result.Blocks[0].Issues, "script:pre")
	if issue == nil || issue.Severity != SeverityError {
		t.Fatalf("issue = %+v, want an error for a malformed script marker", issue)
	}
}

func TestAnalyzeInvalidLuaSyntaxErrors(t *testing.T) {
	src := "###\n# @name Get\n< {%\n  this is not valid lua (((\n%}\n\nGET http://localhost:8080/get HTTP/1.1\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	issue := findIssue(result.Blocks[0].Issues, "script:pre")
	if issue == nil || issue.Severity != SeverityError {
		t.Fatalf("issue = %+v, want an error for invalid lua syntax", issue)
	}
}

func TestAnalyzeLangLuaIsSilent(t *testing.T) {
	src := "###\n# @name Get\n# @lang lua\nGET http://localhost:8080/get HTTP/1.1\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if issue := findIssue(result.Blocks[0].Issues, "metadata:lang"); issue != nil {
		t.Errorf("issue = %+v, want no issue for @lang lua", issue)
	}
}

func TestAnalyzeLangOtherThanLuaWarnsAndFallsBack(t *testing.T) {
	src := "###\n# @name Get\n# @lang javascript\nGET http://localhost:8080/get HTTP/1.1\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	issue := findIssue(result.Blocks[0].Issues, "metadata:lang")
	if issue == nil || issue.Severity != SeverityWarning {
		t.Fatalf("issue = %+v, want a warning", issue)
	}
	if result.Blocks[0].Request.Lang != "lua" {
		t.Errorf("Lang = %q, want fallback to %q", result.Blocks[0].Request.Lang, "lua")
	}
}

func TestAnalyzePostScriptBeforeRequestLineErrors(t *testing.T) {
	src := "###\n# @name Get\n> {%\n  client.global:set(\"x\", 1)\n%}\nGET http://localhost:8080/get HTTP/1.1\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	issue := findIssue(result.Blocks[0].Issues, "request-line")
	if issue == nil || issue.Severity != SeverityError {
		t.Fatalf("issue = %+v, want an error for a post-script placed before the request line", issue)
	}
}

func TestAnalyzeLocalVariableOverridesGlobalWithoutBleeding(t *testing.T) {
	src := "@env = prod\n\n" +
		"###\n# @name UsesLocal\n@env = dev\nGET http://localhost:8080/get?env={{env}} HTTP/1.1\n\n" +
		"###\n# @name UsesGlobal\nGET http://localhost:8080/get?env={{env}} HTTP/1.1\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.HasErrors() {
		t.Fatalf("HasErrors() = true, want false: %+v", result.Blocks)
	}
	if want, got := "http://localhost:8080/get?env=dev", result.Blocks[0].Request.URL; got != want {
		t.Errorf("block 0 URL = %q, want %q (local override)", got, want)
	}
	if want, got := "http://localhost:8080/get?env=prod", result.Blocks[1].Request.URL; got != want {
		t.Errorf("block 1 URL = %q, want %q (global value, not the other block's local)", got, want)
	}
}

func TestAnalyzeNameNotAllowedAsGlobalMetadata(t *testing.T) {
	src := "# @name GlobalName\n\n###\n# @name Real\nGET http://localhost:8080/get HTTP/1.1\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	issue := findIssue(result.GlobalIssues, "metadata:name")
	if issue == nil || issue.Severity != SeverityError {
		t.Fatalf("GlobalIssues = %+v, want an error for @name in the prelude", result.GlobalIssues)
	}
	if !result.HasErrors() {
		t.Errorf("HasErrors() = false, want true")
	}
	// The real per-request block should be unaffected by the prelude error.
	if result.Blocks[0].HasErrors() {
		t.Errorf("Blocks[0].Issues = %+v, want none", result.Blocks[0].Issues)
	}
}

func TestAnalyzeGlobalProxyAppliesToEveryRequestUnlessOverridden(t *testing.T) {
	src := "# @proxy http://global-proxy:8888\n\n" +
		"###\n# @name Default\nGET http://localhost:8080/get HTTP/1.1\n\n" +
		"###\n# @name Overridden\n# @proxy http://local-proxy:9999\nGET http://localhost:8080/get HTTP/1.1\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.HasErrors() {
		t.Fatalf("HasErrors() = true, want false: global=%+v blocks=%+v", result.GlobalIssues, result.Blocks)
	}
	if want, got := "http://global-proxy:8888", result.Blocks[0].Request.Proxy; got != want {
		t.Errorf("Default proxy = %q, want %q", got, want)
	}
	if want, got := "http://local-proxy:9999", result.Blocks[1].Request.Proxy; got != want {
		t.Errorf("Overridden proxy = %q, want %q", got, want)
	}
}

func TestAnalyzeInvalidProxyErrors(t *testing.T) {
	src := "###\n# @name Get\n# @proxy localhost:3128\nGET http://localhost:8080/get HTTP/1.1\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	issue := findIssue(result.Blocks[0].Issues, "metadata:proxy")
	if issue == nil || issue.Severity != SeverityError {
		t.Fatalf("issue = %+v, want an error for a proxy URL missing scheme/host", issue)
	}
}

func TestAnalyzePreludeStrayContentWarns(t *testing.T) {
	src := "this is not a variable or metadata line\n\n###\n# @name Get\nGET http://localhost:8080/get HTTP/1.1\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	issue := findIssue(result.GlobalIssues, "global")
	if issue == nil || issue.Severity != SeverityWarning {
		t.Fatalf("GlobalIssues = %+v, want a warning for stray prelude content", result.GlobalIssues)
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

func TestAnalyzeWithEnvPrecedence(t *testing.T) {
	// host: file-global default, overridden by the environment.
	// greeting: only in the environment.
	// path: only local to the block, overriding nothing.
	src := "@host = http://localhost:8080\n\n" +
		"###\n# @name Get\n@path = get\nGET {{host}}/{{path}}?greeting={{greeting}} HTTP/1.1\n"
	envVars := map[string]string{"host": "https://api.example.com", "greeting": "hi"}

	result, err := AnalyzeWithEnv(strings.NewReader(src), envVars)
	if err != nil {
		t.Fatalf("AnalyzeWithEnv: %v", err)
	}
	if result.HasErrors() {
		t.Fatalf("HasErrors() = true, want false: %+v", result.Blocks)
	}
	if want, got := "https://api.example.com/get?greeting=hi", result.Blocks[0].Request.URL; got != want {
		t.Errorf("URL = %q, want %q", got, want)
	}
	if want, got := map[string]string{"host": "https://api.example.com", "greeting": "hi"}, result.Variables; !reflect.DeepEqual(want, got) {
		t.Errorf("Variables = %+v, want %+v", got, want)
	}
}

func TestAnalyzeWithEnvLocalVariableStillWinsOverEnv(t *testing.T) {
	src := "###\n# @name Get\n@host = http://local-override\nGET {{host}}/get HTTP/1.1\n"
	envVars := map[string]string{"host": "https://api.example.com"}

	result, err := AnalyzeWithEnv(strings.NewReader(src), envVars)
	if err != nil {
		t.Fatalf("AnalyzeWithEnv: %v", err)
	}
	if want, got := "http://local-override/get", result.Blocks[0].Request.URL; got != want {
		t.Errorf("URL = %q, want %q (local should win over environment)", got, want)
	}
}

func TestAnalyzeNilEnvVarsMatchesAnalyze(t *testing.T) {
	src := "###\n# @name Get\nGET http://localhost:8080/get HTTP/1.1\n"
	withEnv, err := AnalyzeWithEnv(strings.NewReader(src), nil)
	if err != nil {
		t.Fatalf("AnalyzeWithEnv: %v", err)
	}
	without, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !reflect.DeepEqual(withEnv, without) {
		t.Errorf("AnalyzeWithEnv(nil) = %+v, want %+v", withEnv, without)
	}
}

func TestAnalyzeDuplicateNameAcrossBlocksErrors(t *testing.T) {
	src := "###\n# @name Dup\nGET http://localhost:8080/first HTTP/1.1\n\n###\n# @name Dup\nGET http://localhost:8080/second HTTP/1.1\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(result.Blocks) != 2 {
		t.Fatalf("got %d blocks, want 2", len(result.Blocks))
	}
	if issue := findIssue(result.Blocks[0].Issues, "metadata:name"); issue != nil {
		t.Errorf("first block unexpectedly has a name issue: %+v", *issue)
	}
	issue := findIssue(result.Blocks[1].Issues, "metadata:name")
	if issue == nil || issue.Severity != SeverityError || !strings.Contains(issue.Message, "duplicate @name") {
		t.Fatalf("second block issue = %+v, want a duplicate @name error", issue)
	}
	if _, err := Parse(strings.NewReader(src)); err == nil {
		t.Errorf("Parse succeeded, want an error for the duplicate @name")
	}
}

func TestAnalyzeCommentLinesAmongHeadersAreSkipped(t *testing.T) {
	src := strings.Join([]string{
		"###",
		"# @name Commented",
		"GET http://localhost:8080/get HTTP/1.1",
		"# a note about the next header",
		"Accept: application/json",
		"# Authorization: Bearer disabled-for-now",
		"X-Last: yes",
		"",
		"# this is body, not a comment",
	}, "\n")
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	block := result.Blocks[0]
	if len(block.Issues) != 0 {
		t.Fatalf("issues = %+v, want none", block.Issues)
	}
	var names []string
	for _, h := range block.Request.Headers {
		names = append(names, h.Name)
	}
	if want := []string{"Accept", "X-Last"}; !reflect.DeepEqual(names, want) {
		t.Errorf("headers = %v, want %v", names, want)
	}
	if want := "# this is body, not a comment"; block.Request.Body != want {
		t.Errorf("body = %q, want %q", block.Request.Body, want)
	}
}

func TestAnalyzeMetadataAmongHeadersErrors(t *testing.T) {
	src := "###\nGET http://localhost:8080/get HTTP/1.1\n# @name TooLate\nAccept: */*\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	var found bool
	for _, issue := range result.Blocks[0].Issues {
		if issue.Element == "metadata:name" && issue.Severity == SeverityError && strings.Contains(issue.Message, "before the request line") {
			found = true
		}
	}
	if !found {
		t.Errorf("issues = %+v, want a misplaced-metadata error", result.Blocks[0].Issues)
	}
	if result.Blocks[0].Request.Name != "" {
		t.Errorf("name = %q, want it not to be picked up from after the request line", result.Blocks[0].Request.Name)
	}
}
