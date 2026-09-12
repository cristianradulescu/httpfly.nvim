package curl

import (
	"strings"
	"testing"

	"github.com/cristianradulescu/httpfly/internal/httpfile"
)

func TestToCurlBasic(t *testing.T) {
	req := httpfile.Request{
		Method:  "GET",
		URL:     "https://example.com/get",
		Headers: []httpfile.Header{{Name: "Accept", Value: "application/json"}},
	}
	want := "curl 'https://example.com/get' \\\n" +
		"  -X 'GET' \\\n" +
		"  -H 'Accept: application/json'\n"
	if got := ToCurl(req); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestToCurlWithBodyProxyAndMultipleHeadersPreservesOrder(t *testing.T) {
	req := httpfile.Request{
		Method: "POST",
		URL:    "https://example.com/post",
		Headers: []httpfile.Header{
			{Name: "Content-Type", Value: "application/json"},
			{Name: "Authorization", Value: "Bearer abc123"},
		},
		Proxy: "http://localhost:3128",
		Body:  `{"a":1}`,
	}
	got := ToCurl(req)
	want := "curl 'https://example.com/post' \\\n" +
		"  -X 'POST' \\\n" +
		"  -H 'Content-Type: application/json' \\\n" +
		"  -H 'Authorization: Bearer abc123' \\\n" +
		"  -x 'http://localhost:3128' \\\n" +
		`  --data-raw '{"a":1}'` + "\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestToCurlNoHeadersOrBody(t *testing.T) {
	req := httpfile.Request{Method: "GET", URL: "https://example.com"}
	want := "curl 'https://example.com' \\\n  -X 'GET'\n"
	if got := ToCurl(req); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestShellQuoteEscapesEmbeddedSingleQuote(t *testing.T) {
	got := shellQuote(`it's a test`)
	want := `'it'\''s a test'`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestToCurlEmbeddedSingleQuoteInHeaderRoundTrips(t *testing.T) {
	req := httpfile.Request{
		Method:  "GET",
		URL:     "https://example.com",
		Headers: []httpfile.Header{{Name: "X-Test", Value: `it's here`}},
	}
	curlCmd := ToCurl(req)
	if !strings.Contains(curlCmd, `'\''`) {
		t.Fatalf("expected escaped single quote in output: %q", curlCmd)
	}

	reparsed, warnings, err := Parse(curlCmd)
	if err != nil {
		t.Fatalf("Parse(ToCurl(req)): %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}
	if len(reparsed.Headers) != 1 || reparsed.Headers[0].Value != `it's here` {
		t.Errorf("Headers = %+v, want the original value to round-trip", reparsed.Headers)
	}
}

// TestRoundTripThroughCurlAndBack takes a curl command, converts it to an
// httpfile.Request, renders that back out as curl, and reparses the
// result -- the reconstructed request must carry the same method, URL,
// headers, and body as the original, even if the literal text differs
// (e.g. method now explicit, flag choice differs).
func TestRoundTripThroughCurlAndBack(t *testing.T) {
	original := `curl 'https://api.example.com/users' -X PUT -H 'Content-Type: application/json' -H 'Authorization: Bearer abc123' -x 'http://localhost:3128' --data-raw '{"name":"Ada","note":"it'"'"'s fine"}'`

	first, warnings, err := Parse(original)
	if err != nil {
		t.Fatalf("Parse(original): %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}

	regenerated := ToCurl(first)

	second, warnings, err := Parse(regenerated)
	if err != nil {
		t.Fatalf("Parse(regenerated): %v\ngenerated command:\n%s", err, regenerated)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}

	if first.Method != second.Method {
		t.Errorf("Method: first=%q second=%q", first.Method, second.Method)
	}
	if first.URL != second.URL {
		t.Errorf("URL: first=%q second=%q", first.URL, second.URL)
	}
	if first.Proxy != second.Proxy {
		t.Errorf("Proxy: first=%q second=%q", first.Proxy, second.Proxy)
	}
	if first.Body != second.Body {
		t.Errorf("Body: first=%q second=%q", first.Body, second.Body)
	}
	if len(first.Headers) != len(second.Headers) {
		t.Fatalf("Headers count: first=%d second=%d", len(first.Headers), len(second.Headers))
	}
	for i := range first.Headers {
		if first.Headers[i] != second.Headers[i] {
			t.Errorf("Headers[%d]: first=%+v second=%+v", i, first.Headers[i], second.Headers[i])
		}
	}
}

func TestToCurlWholeBodyFileReferenceUsesDataBinary(t *testing.T) {
	req := httpfile.Request{
		Method:  "PUT",
		URL:     "https://example.com/put",
		Headers: []httpfile.Header{{Name: "Content-Type", Value: "image/png"}},
		Body:    "\x89PNGbinarydata",
		RawBody: "< ./photo.png",
	}
	got := ToCurl(req)
	want := "curl 'https://example.com/put' \\\n" +
		"  -X 'PUT' \\\n" +
		"  -H 'Content-Type: image/png' \\\n" +
		"  --data-binary '@./photo.png'\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestToCurlMultipartBodyReconstructsFormFlags(t *testing.T) {
	req := httpfile.Request{
		Method: "POST",
		URL:    "https://example.com/post",
		Headers: []httpfile.Header{
			{Name: "Content-Type", Value: "multipart/form-data; boundary=B"},
		},
		RawBody: "--B\n" +
			"Content-Disposition: form-data; name=\"avatar\"; filename=\"photo.png\"\n" +
			"Content-Type: image/png\n\n" +
			"< ./photo.png\n" +
			"--B\n" +
			"Content-Disposition: form-data; name=\"username\"\n\n" +
			"alice\n" +
			"--B--",
	}
	got := ToCurl(req)
	want := "curl 'https://example.com/post' \\\n" +
		"  -X 'POST' \\\n" +
		"  -F 'avatar=@./photo.png;type=image/png' \\\n" +
		"  -F 'username=alice'\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestRoundTripFileUploadThroughCurlAndBack mirrors
// TestRoundTripThroughCurlAndBack but for a "-F" file-upload command --
// Parse must turn it into httpfly's "< path" body syntax, and ToCurl must
// turn that back into an equivalent "-F" command, not inline any content.
func TestRoundTripFileUploadThroughCurlAndBack(t *testing.T) {
	original := `curl 'https://example.com/upload' -F 'avatar=@photo.png;type=image/png' -F 'username=alice'`

	first, warnings, err := Parse(original)
	if err != nil {
		t.Fatalf("Parse(original): %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}

	regenerated := ToCurl(first)
	if !strings.Contains(regenerated, "-F") {
		t.Fatalf("regenerated = %q, want it to use -F, not inline the multipart body", regenerated)
	}

	second, warnings, err := Parse(regenerated)
	if err != nil {
		t.Fatalf("Parse(regenerated): %v\ngenerated command:\n%s", err, regenerated)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}
	if first.Body != second.Body {
		t.Errorf("Body did not round-trip:\nfirst:  %q\nsecond: %q", first.Body, second.Body)
	}
}
