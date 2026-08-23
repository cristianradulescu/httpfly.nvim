package curl

import (
	"strings"
	"testing"

	"github.com/cristianradulescu/httpfly/internal/httpfile"
)

func TestFormatGetNoBody(t *testing.T) {
	req := httpfile.Request{
		Name:    "Get",
		Method:  "GET",
		URL:     "https://example.com/get",
		Proto:   "HTTP/1.1",
		Headers: []httpfile.Header{{Name: "Accept", Value: "application/json"}},
	}
	want := "###\n# @name Get\nGET https://example.com/get HTTP/1.1\nAccept: application/json\n"
	if got := Format(req); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatWithBody(t *testing.T) {
	req := httpfile.Request{
		Name:    "Post",
		Method:  "POST",
		URL:     "https://example.com/post",
		Proto:   "HTTP/1.1",
		Headers: []httpfile.Header{{Name: "Content-Type", Value: "application/json"}},
		Body:    `{"a":1}`,
	}
	got := Format(req)
	if !strings.HasSuffix(got, "\n\n{\"a\":1}\n") {
		t.Errorf("got %q, want it to end with a blank line then the body", got)
	}
}

func TestFormatWithProxy(t *testing.T) {
	req := httpfile.Request{
		Name:   "Get",
		Method: "GET",
		URL:    "https://example.com",
		Proto:  "HTTP/1.1",
		Proxy:  "http://localhost:3128",
	}
	got := Format(req)
	if !strings.Contains(got, "# @proxy http://localhost:3128\n") {
		t.Errorf("got %q, want it to contain \"# @proxy ...\"", got)
	}
}

func TestFormatDefaultsProtoWhenUnset(t *testing.T) {
	req := httpfile.Request{Name: "Get", Method: "GET", URL: "https://example.com"}
	got := Format(req)
	if !strings.Contains(got, "GET https://example.com HTTP/1.1\n") {
		t.Errorf("got %q, want HTTP/1.1 default", got)
	}
}
