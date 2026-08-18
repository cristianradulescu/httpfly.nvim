package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cristianradulescu/httpfly/internal/httpfile"
)

func TestSendGetRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept header = %q, want application/json", got)
		}
		w.Header().Set("X-Test", "yes")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	req := httpfile.Request{
		Method:  "GET",
		URL:     srv.URL + "/get",
		Proto:   "HTTP/1.1",
		Headers: []httpfile.Header{{Name: "Accept", Value: "application/json"}},
	}

	result := New().Send(context.Background(), req)
	if result.Err != nil {
		t.Fatalf("Send: %v", result.Err)
	}
	if result.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want 200", result.StatusCode)
	}
	if string(result.Body) != `{"ok":true}` {
		t.Errorf("Body = %q", result.Body)
	}
	if got := result.Headers.Get("X-Test"); got != "yes" {
		t.Errorf("X-Test header = %q, want yes", got)
	}
}

func TestSendPostWithBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"name":"John"}` {
			t.Errorf("body = %q", body)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	req := httpfile.Request{
		Method:  "POST",
		URL:     srv.URL + "/post",
		Headers: []httpfile.Header{{Name: "Content-Type", Value: "application/json"}},
		Body:    `{"name":"John"}`,
	}

	result := New().Send(context.Background(), req)
	if result.Err != nil {
		t.Fatalf("Send: %v", result.Err)
	}
	if result.StatusCode != http.StatusCreated {
		t.Errorf("StatusCode = %d, want 201", result.StatusCode)
	}
}

func TestSendConnectionError(t *testing.T) {
	req := httpfile.Request{Method: "GET", URL: "http://127.0.0.1:1"}
	result := New().Send(context.Background(), req)
	if result.Err == nil {
		t.Fatal("expected a connection error, got nil")
	}
}

func TestSendThroughProxy(t *testing.T) {
	var proxied bool
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxied = true
		// A forward proxy receives the absolute-URI request line unmodified.
		if !r.URL.IsAbs() {
			t.Errorf("request URL = %q, want an absolute URI (proxy semantics)", r.URL)
		}
		w.WriteHeader(http.StatusTeapot)
	}))
	defer proxy.Close()

	req := httpfile.Request{
		Method: "GET",
		URL:    "http://example.invalid/get",
		Proxy:  proxy.URL,
	}

	result := New().Send(context.Background(), req)
	if result.Err != nil {
		t.Fatalf("Send: %v", result.Err)
	}
	if !proxied {
		t.Fatal("request never reached the proxy")
	}
	if result.StatusCode != http.StatusTeapot {
		t.Errorf("StatusCode = %d, want 418 (from the proxy, not example.invalid)", result.StatusCode)
	}
}

func TestSendInvalidProxyURL(t *testing.T) {
	req := httpfile.Request{Method: "GET", URL: "http://example.invalid/get", Proxy: "://not a url"}
	result := New().Send(context.Background(), req)
	if result.Err == nil {
		t.Fatal("expected an error for an invalid proxy URL")
	}
}
