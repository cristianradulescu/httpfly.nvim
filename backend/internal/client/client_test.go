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

func TestSendFollowsRedirectAndReportsFinalURL(t *testing.T) {
	var targetURL string
	mux := http.NewServeMux()
	mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, targetURL, http.StatusFound)
	})
	mux.HandleFunc("/end", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	targetURL = srv.URL + "/end"

	req := httpfile.Request{Method: "GET", URL: srv.URL + "/start"}
	result := New().Send(context.Background(), req)
	if result.Err != nil {
		t.Fatalf("Send: %v", result.Err)
	}
	if result.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want 200 (from the redirect target)", result.StatusCode)
	}
	if result.FinalURL != targetURL {
		t.Errorf("FinalURL = %q, want %q", result.FinalURL, targetURL)
	}
}

func TestSendNoRedirectFinalURLMatchesRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	req := httpfile.Request{Method: "GET", URL: srv.URL + "/get"}
	result := New().Send(context.Background(), req)
	if result.Err != nil {
		t.Fatalf("Send: %v", result.Err)
	}
	if result.FinalURL != req.URL {
		t.Errorf("FinalURL = %q, want %q (no redirect)", result.FinalURL, req.URL)
	}
}

func TestSendPlainHTTPHasNoTLSState(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	req := httpfile.Request{Method: "GET", URL: srv.URL + "/get"}
	result := New().Send(context.Background(), req)
	if result.Err != nil {
		t.Fatalf("Send: %v", result.Err)
	}
	if result.TLS != nil {
		t.Errorf("TLS = %+v, want nil for a plain HTTP request", result.TLS)
	}
}

func TestSendHTTPSCapturesTLSState(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New()
	c.HTTP = srv.Client() // trust the test server's self-signed cert

	req := httpfile.Request{Method: "GET", URL: srv.URL + "/get"}
	result := c.Send(context.Background(), req)
	if result.Err != nil {
		t.Fatalf("Send: %v", result.Err)
	}
	if result.TLS == nil {
		t.Fatal("TLS = nil, want a populated ConnectionState for an HTTPS request")
	}
}
