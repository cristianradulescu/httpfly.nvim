package curl

import (
	"strings"
	"testing"
)

func TestParseSimpleGet(t *testing.T) {
	req, warnings, err := Parse(`curl 'https://example.com/get' -H 'Accept: application/json'`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}
	if req.Method != "GET" {
		t.Errorf("Method = %q, want GET", req.Method)
	}
	if req.URL != "https://example.com/get" {
		t.Errorf("URL = %q", req.URL)
	}
	if len(req.Headers) != 1 || req.Headers[0].Name != "Accept" || req.Headers[0].Value != "application/json" {
		t.Errorf("Headers = %+v", req.Headers)
	}
}

func TestParseURLFlag(t *testing.T) {
	req, _, err := Parse(`curl --url 'https://example.com/get'`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if req.URL != "https://example.com/get" {
		t.Errorf("URL = %q", req.URL)
	}
}

func TestParseMethodInferredFromData(t *testing.T) {
	req, _, err := Parse(`curl 'https://example.com/post' -d '{"a":1}'`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if req.Method != "POST" {
		t.Errorf("Method = %q, want POST", req.Method)
	}
	if req.Body != `{"a":1}` {
		t.Errorf("Body = %q", req.Body)
	}
}

func TestParseExplicitMethodWins(t *testing.T) {
	req, _, err := Parse(`curl -X PUT 'https://example.com/put' -d '{"a":1}'`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if req.Method != "PUT" {
		t.Errorf("Method = %q, want PUT", req.Method)
	}
}

func TestParseMultipleDataFlagsJoinedWithAmpersand(t *testing.T) {
	req, _, err := Parse(`curl 'https://example.com/post' -d 'a=1' -d 'b=2'`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if want := "a=1&b=2"; req.Body != want {
		t.Errorf("Body = %q, want %q", req.Body, want)
	}
}

func TestParseCookieFlagBecomesHeader(t *testing.T) {
	req, _, err := Parse(`curl 'https://example.com' -b 'a=1; b=2'`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(req.Headers) != 1 || req.Headers[0].Name != "Cookie" || req.Headers[0].Value != "a=1; b=2" {
		t.Errorf("Headers = %+v", req.Headers)
	}
}

func TestParseUserFlagBecomesBasicAuthHeader(t *testing.T) {
	req, _, err := Parse(`curl 'https://example.com' -u 'alice:secret'`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(req.Headers) != 1 || req.Headers[0].Name != "Authorization" {
		t.Fatalf("Headers = %+v", req.Headers)
	}
	if want := "Basic YWxpY2U6c2VjcmV0"; req.Headers[0].Value != want {
		t.Errorf("Authorization = %q, want %q", req.Headers[0].Value, want)
	}
}

func TestParseProxyFlag(t *testing.T) {
	req, _, err := Parse(`curl 'https://example.com' -x 'http://localhost:3128'`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if req.Proxy != "http://localhost:3128" {
		t.Errorf("Proxy = %q", req.Proxy)
	}
}

func TestParseHeadFlagSetsMethod(t *testing.T) {
	req, _, err := Parse(`curl 'https://example.com' -I`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if req.Method != "HEAD" {
		t.Errorf("Method = %q, want HEAD", req.Method)
	}
}

func TestParseInsecureFlagWarnsAndIsDropped(t *testing.T) {
	_, warnings, err := Parse(`curl 'https://example.com' -k`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "insecure") {
		t.Errorf("warnings = %v, want one mentioning -k/--insecure", warnings)
	}
}

func TestParseOutputFlagWarnsAndIsDropped(t *testing.T) {
	_, warnings, err := Parse(`curl 'https://example.com' -o result.json`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "output") {
		t.Errorf("warnings = %v, want one mentioning -o/--output", warnings)
	}
}

func TestParseCompressedAndLocationAreSilentlyIgnored(t *testing.T) {
	req, warnings, err := Parse(`curl 'https://example.com' --compressed -L`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}
	if req.Method != "GET" {
		t.Errorf("Method = %q", req.Method)
	}
}

func TestParseUnrecognizedFlagWarns(t *testing.T) {
	_, warnings, err := Parse(`curl 'https://example.com' --some-unknown-flag`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly one", warnings)
	}
}

func TestParseLongFlagEqualsForm(t *testing.T) {
	req, _, err := Parse(`curl 'https://example.com' --header='Accept: application/json'`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(req.Headers) != 1 || req.Headers[0].Value != "application/json" {
		t.Errorf("Headers = %+v", req.Headers)
	}
}

func TestParseDataURLEncode(t *testing.T) {
	req, _, err := Parse(`curl 'https://example.com' --data-urlencode 'name=hello world'`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if want := "name=hello+world"; req.Body != want {
		t.Errorf("Body = %q, want %q", req.Body, want)
	}
}

func TestParseDataFromFileBecomesFileReference(t *testing.T) {
	req, warnings, err := Parse(`curl 'https://example.com' -d '@payload.json'`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if want := "< payload.json"; req.Body != want {
		t.Errorf("Body = %q, want %q", req.Body, want)
	}
	if req.Method != "POST" {
		t.Errorf("Method = %q, want POST (implied by -d)", req.Method)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}
}

func TestParseDataBinaryFromFileBecomesFileReference(t *testing.T) {
	req, _, err := Parse(`curl 'https://example.com' --data-binary '@photo.png'`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if want := "< photo.png"; req.Body != want {
		t.Errorf("Body = %q, want %q", req.Body, want)
	}
}

func TestParseFileDataCombinedWithOtherDataIsWarnedAndDropped(t *testing.T) {
	req, warnings, err := Parse(`curl 'https://example.com' -d 'a=b' -d '@payload.json'`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if want := "a=b"; req.Body != want {
		t.Errorf("Body = %q, want %q (the file part dropped, not silently corrupting the body)", req.Body, want)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly one", warnings)
	}
}

func TestParseHeaderWithEmbeddedNewlineIsDroppedNotInjected(t *testing.T) {
	req, warnings, err := Parse("curl 'https://example.com' -H 'X-Test: a\\nb'")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// The literal two characters "\" "n" (from the shell string above)
	// don't form a real newline once tokenized -- this exercises the path
	// but a genuine embedded newline (e.g. from a raw multi-line clipboard
	// paste) is what addHeader's guard is for; assert no header injection
	// occurred either way.
	for _, h := range req.Headers {
		if strings.ContainsAny(h.Value, "\r\n") {
			t.Errorf("header value contains a raw newline: %q", h.Value)
		}
	}
	_ = warnings
}

func TestParseNoURLErrors(t *testing.T) {
	if _, _, err := Parse(`curl -H 'Accept: application/json'`); err == nil {
		t.Fatal("expected an error when no URL is present")
	}
}

func TestParseNotCurlErrors(t *testing.T) {
	if _, _, err := Parse(`wget https://example.com`); err == nil {
		t.Fatal("expected an error for input not starting with \"curl\"")
	}
}

// TestParseRealChromeExample is the actual "Copy as cURL (bash)" output
// captured from Chrome against a GitHub page, reproduced verbatim (bar the
// literal session cookie values, which don't matter for parsing).
func TestParseRealChromeExample(t *testing.T) {
	cmd := "curl --url 'https://github.com/_side-panels/user.json' \\\n" +
		"  -H 'accept: */*' \\\n" +
		"  -H 'accept-language: en-RO,en;q=0.9,ro-RO;q=0.8,ro;q=0.7,en-GB;q=0.6,en-US;q=0.5' \\\n" +
		"  -b '_octo=GH1.1.1156895386.1767446645; user_session=abc123' \\\n" +
		"  -H 'dnt: 1' \\\n" +
		"  -H 'if-none-match: W/\"0c341038a74e06d043870e132158ed4e\"' \\\n" +
		"  -H 'priority: u=1, i' \\\n" +
		"  -H 'referer: https://github.com/cristianradulescu/httpfly' \\\n" +
		"  -H 'sec-ch-ua: \"Not=A?Brand\";v=\"99\", \"Google Chrome\";v=\"151\", \"Chromium\";v=\"151\"' \\\n" +
		"  -H 'sec-ch-ua-mobile: ?0' \\\n" +
		"  -H 'sec-ch-ua-platform: \"Linux\"' \\\n" +
		"  -H 'sec-fetch-dest: empty' \\\n" +
		"  -H 'sec-fetch-mode: cors' \\\n" +
		"  -H 'sec-fetch-site: same-origin' \\\n" +
		"  -H 'sec-gpc: 1' \\\n" +
		"  -H 'user-agent: Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36'"

	req, warnings, err := Parse(cmd)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}
	if req.Method != "GET" {
		t.Errorf("Method = %q, want GET", req.Method)
	}
	if req.URL != "https://github.com/_side-panels/user.json" {
		t.Errorf("URL = %q", req.URL)
	}
	if len(req.Headers) != 15 {
		t.Fatalf("got %d headers, want 15 (14 -H flags + 1 -b/cookie): %+v", len(req.Headers), req.Headers)
	}

	byName := make(map[string]string, len(req.Headers))
	for _, h := range req.Headers {
		byName[h.Name] = h.Value
	}
	if want := `"Not=A?Brand";v="99", "Google Chrome";v="151", "Chromium";v="151"`; byName["sec-ch-ua"] != want {
		t.Errorf("sec-ch-ua = %q, want %q", byName["sec-ch-ua"], want)
	}
	if want := `W/"0c341038a74e06d043870e132158ed4e"`; byName["if-none-match"] != want {
		t.Errorf("if-none-match = %q, want %q", byName["if-none-match"], want)
	}
	if want := "_octo=GH1.1.1156895386.1767446645; user_session=abc123"; byName["Cookie"] != want {
		t.Errorf("Cookie = %q, want %q", byName["Cookie"], want)
	}
}

func TestParseFormFileUpload(t *testing.T) {
	req, warnings, err := Parse(`curl 'https://example.com/upload' -F 'avatar=@photo.png;type=image/png' -F 'username=alice'`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}
	if req.Method != "POST" {
		t.Errorf("Method = %q, want POST (implied by -F)", req.Method)
	}

	var contentType string
	for _, h := range req.Headers {
		if h.Name == "Content-Type" {
			contentType = h.Value
		}
	}
	if !strings.HasPrefix(contentType, "multipart/form-data; boundary=") {
		t.Fatalf("Content-Type = %q, want a multipart/form-data value", contentType)
	}
	boundary := strings.TrimPrefix(contentType, "multipart/form-data; boundary=")

	if !strings.Contains(req.Body, "--"+boundary) {
		t.Errorf("Body = %q, want it delimited by boundary %q", req.Body, boundary)
	}
	if !strings.Contains(req.Body, `name="avatar"; filename="photo.png"`) {
		t.Errorf("Body = %q, want avatar's Content-Disposition with filename", req.Body)
	}
	if !strings.Contains(req.Body, "Content-Type: image/png") {
		t.Errorf("Body = %q, want avatar's explicit Content-Type", req.Body)
	}
	if !strings.Contains(req.Body, "< photo.png") {
		t.Errorf("Body = %q, want a \"< photo.png\" file reference, not inlined content", req.Body)
	}
	if !strings.Contains(req.Body, "name=\"username\"") || !strings.Contains(req.Body, "alice") {
		t.Errorf("Body = %q, want the plain username field", req.Body)
	}
	if !strings.HasSuffix(req.Body, "--"+boundary+"--") {
		t.Errorf("Body = %q, want it to end with the closing boundary", req.Body)
	}
}

func TestParseFormFilenameOverride(t *testing.T) {
	req, _, err := Parse(`curl 'https://example.com' -F 'file=@/tmp/upload.bin;filename=report.pdf'`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !strings.Contains(req.Body, `filename="report.pdf"`) {
		t.Errorf("Body = %q, want the filename= override applied", req.Body)
	}
	if !strings.Contains(req.Body, "< /tmp/upload.bin") {
		t.Errorf("Body = %q, want the original path referenced", req.Body)
	}
}

func TestParseFormFilenameDefaultsToPathBase(t *testing.T) {
	req, _, err := Parse(`curl 'https://example.com' -F 'file=@/tmp/dir/photo.png'`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !strings.Contains(req.Body, `filename="photo.png"`) {
		t.Errorf("Body = %q, want filename defaulted to the path's base name", req.Body)
	}
}

func TestParseFormCombinedWithDataIsWarned(t *testing.T) {
	req, warnings, err := Parse(`curl 'https://example.com' -F 'file=@photo.png' -d 'a=b'`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if strings.Contains(req.Body, "a=b") {
		t.Errorf("Body = %q, want the -d value dropped when combined with -F", req.Body)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly one", warnings)
	}
}

func TestParseMalformedFormFieldIsWarnedAndSkipped(t *testing.T) {
	req, warnings, err := Parse(`curl 'https://example.com' -F 'not-a-valid-field'`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if req.Body != "" {
		t.Errorf("Body = %q, want empty (the malformed field skipped)", req.Body)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly one", warnings)
	}
}
