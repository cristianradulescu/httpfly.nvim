package parser

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/cristianradulescu/httpfly/internal/httpfile"
)

func TestParseBasicExample(t *testing.T) {
	f, err := os.Open("../../doc/examples/1_basic.http")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	got, err := Parse(f)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	want := []httpfile.Request{
		{
			Name:       "Get",
			Method:     "GET",
			URL:        "http://localhost:8080/get?greeting=hello",
			Proto:      "HTTP/1.1",
			Headers:    []httpfile.Header{{Name: "Accept", Value: "application/json"}},
			Lang:       "lua",
			Variables:  map[string]string{},
			RawURL:     "http://localhost:8080/get?greeting=hello",
			RawHeaders: []httpfile.Header{{Name: "Accept", Value: "application/json"}},
		},
		{
			Name:       "Post",
			Method:     "POST",
			URL:        "http://localhost:8080/post",
			Proto:      "HTTP/1.1",
			Headers:    []httpfile.Header{{Name: "Content-Type", Value: "application/json"}},
			Body:       "{\n  \"name\": \"John\",\n  \"greeting\": \"Hello\"\n}",
			Lang:       "lua",
			Variables:  map[string]string{},
			RawURL:     "http://localhost:8080/post",
			RawHeaders: []httpfile.Header{{Name: "Content-Type", Value: "application/json"}},
			RawBody:    "{\n  \"name\": \"John\",\n  \"greeting\": \"Hello\"\n}",
		},
		{
			Name:       "Name in separator",
			Method:     "GET",
			URL:        "http://localhost:8080/get?greeting=hello-again",
			Proto:      "HTTP/1.1",
			Headers:    []httpfile.Header{{Name: "Accept", Value: "application/json"}},
			Lang:       "lua",
			Variables:  map[string]string{},
			RawURL:     "http://localhost:8080/get?greeting=hello-again",
			RawHeaders: []httpfile.Header{{Name: "Accept", Value: "application/json"}},
		},
	}

	if len(got.Requests) != len(want) {
		t.Fatalf("got %d requests, want %d: %+v", len(got.Requests), len(want), got.Requests)
	}
	for i := range want {
		if !reflect.DeepEqual(got.Requests[i], want[i]) {
			t.Errorf("request %d:\n got: %+v\nwant: %+v", i, got.Requests[i], want[i])
		}
	}
}

func TestParseNoTrailingBody(t *testing.T) {
	src := "###\n# @name Get\nGET http://localhost:8080/get HTTP/1.1\nAccept: application/json\n"
	f, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(f.Requests) != 1 {
		t.Fatalf("got %d requests, want 1", len(f.Requests))
	}
	if f.Requests[0].Body != "" {
		t.Errorf("Body = %q, want empty", f.Requests[0].Body)
	}
}

func TestParseNameFromSeparatorLine(t *testing.T) {
	src := "### GetUsers\nGET http://localhost:8080/get HTTP/1.1\n"
	f, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(f.Requests) != 1 {
		t.Fatalf("got %d requests, want 1", len(f.Requests))
	}
	if got, want := f.Requests[0].Name, "GetUsers"; got != want {
		t.Errorf("Name = %q, want %q", got, want)
	}
}

// A "###"-prefixed line anywhere -- including inside a body -- starts a new
// block, matching JetBrains HTTP Client's own behavior. This is a known,
// accepted trade-off of the plain-text format (see doc/http-file-format.md),
// not something httpfly tries to escape or detect.
func TestParseHashHashHashInsideBodySplitsIntoNewBlock(t *testing.T) {
	src := "###\n# @name Get\nPOST http://localhost:8080/post HTTP/1.1\n\n" +
		"### not a real separator, just markdown in the body\nmore body text\n"
	result, err := Analyze(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(result.Blocks) != 2 {
		t.Fatalf("got %d blocks, want 2 (the body's \"###\" line splits into an extra block): %+v", len(result.Blocks), result.Blocks)
	}
	if got, want := result.Blocks[0].Request.Body, ""; got != want {
		t.Errorf("first block Body = %q, want %q (the \"###\" line and everything after it split off)", got, want)
	}
}

func TestParseNoSeparatorSingleRequest(t *testing.T) {
	src := "# @name Get\nGET http://localhost:8080/get HTTP/1.1\n"
	f, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(f.Requests) != 1 {
		t.Fatalf("got %d requests, want 1", len(f.Requests))
	}
}

func TestParseDefaultsProtoWhenOmitted(t *testing.T) {
	src := "###\n# @name Get\nGET http://localhost:8080/get\n"
	f, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := f.Requests[0].Proto; got != "HTTP/1.1" {
		t.Errorf("Proto = %q, want HTTP/1.1", got)
	}
}

func TestParseInvalidRequestLine(t *testing.T) {
	src := "###\nnot a request line\n"
	if _, err := Parse(strings.NewReader(src)); err == nil {
		t.Fatal("expected error for invalid request line, got nil")
	}
}

func TestParseInvalidHeaderLine(t *testing.T) {
	src := "###\nGET http://localhost:8080/get HTTP/1.1\nnot-a-header\n"
	if _, err := Parse(strings.NewReader(src)); err == nil {
		t.Fatal("expected error for invalid header line, got nil")
	}
}

func TestParseEmptyBlocksIgnored(t *testing.T) {
	src := "###\n###\n# @name Get\nGET http://localhost:8080/get HTTP/1.1\n###\n"
	f, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(f.Requests) != 1 {
		t.Fatalf("got %d requests, want 1: %+v", len(f.Requests), f.Requests)
	}
}
