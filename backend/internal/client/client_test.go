package client

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cristianradulescu/httpfly.nvim/backend/internal/httpfile"
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

// writeServerCA writes srv's (self-signed) certificate as a PEM file, for
// use as a request's CACertificate.
func writeServerCA(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ca.pem")
	data := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// writeClientCert generates a self-signed client certificate, writing the
// certificate and key to separate PEM files. The returned pool trusts it.
func writeClientCert(t *testing.T) (certPath, keyPath string, pool *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "httpfly-test-client"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pool = x509.NewCertPool()
	pool.AddCert(cert)

	dir := t.TempDir()
	certPath = filepath.Join(dir, "client.pem")
	keyPath = filepath.Join(dir, "client.key")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	return certPath, keyPath, pool
}

// newMTLSServer starts an HTTPS server that requires a client certificate
// trusted by clientCAs, responding with the presented certificate's CN.
func newMTLSServer(t *testing.T, clientCAs *x509.CertPool) *httptest.Server {
	t.Helper()
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.TLS.PeerCertificates[0].Subject.CommonName)
	}))
	srv.TLS = &tls.Config{ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientCAs}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv
}

func TestSendTrustsCACertificate(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	req := httpfile.Request{Method: "GET", URL: srv.URL}
	if result := New().Send(context.Background(), req); result.Err == nil {
		t.Fatal("expected a verification error without the CA certificate")
	}

	req.Certs = httpfile.Certificates{CACertificate: writeServerCA(t, srv)}
	result := New().Send(context.Background(), req)
	if result.Err != nil {
		t.Fatalf("Send: %v", result.Err)
	}
	if result.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want 200", result.StatusCode)
	}
}

func TestSendPresentsClientCertificate(t *testing.T) {
	certPath, keyPath, pool := writeClientCert(t)
	srv := newMTLSServer(t, pool)
	caPath := writeServerCA(t, srv)

	req := httpfile.Request{Method: "GET", URL: srv.URL, Certs: httpfile.Certificates{CACertificate: caPath}}
	if result := New().Send(context.Background(), req); result.Err == nil {
		t.Fatal("expected the handshake to fail without a client certificate")
	}

	req.Certs.ClientCertificate = certPath
	req.Certs.ClientCertificateKey = keyPath
	result := New().Send(context.Background(), req)
	if result.Err != nil {
		t.Fatalf("Send: %v", result.Err)
	}
	if got := string(result.Body); got != "httpfly-test-client" {
		t.Errorf("Body = %q, want the client certificate's CN", got)
	}
}

func TestSendClientCertificateWithKeyInSameFile(t *testing.T) {
	certPath, keyPath, pool := writeClientCert(t)
	certPEM, _ := os.ReadFile(certPath)
	keyPEM, _ := os.ReadFile(keyPath)
	combined := filepath.Join(t.TempDir(), "combined.pem")
	if err := os.WriteFile(combined, append(certPEM, keyPEM...), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := newMTLSServer(t, pool)

	req := httpfile.Request{Method: "GET", URL: srv.URL, Certs: httpfile.Certificates{
		ClientCertificate: combined,
		CACertificate:     writeServerCA(t, srv),
	}}
	if result := New().Send(context.Background(), req); result.Err != nil {
		t.Fatalf("Send: %v", result.Err)
	}
}

func TestSendInvalidCertificateFiles(t *testing.T) {
	notPEM := filepath.Join(t.TempDir(), "not.pem")
	if err := os.WriteFile(notPEM, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name  string
		certs httpfile.Certificates
		want  string
	}{
		{"missing CA", httpfile.Certificates{CACertificate: filepath.Join(t.TempDir(), "nope.pem")}, "read CA certificate"},
		{"non-PEM CA", httpfile.Certificates{CACertificate: notPEM}, "no PEM certificates found"},
		{"non-PEM client cert", httpfile.Certificates{ClientCertificate: notPEM}, "load client certificate"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httpfile.Request{Method: "GET", URL: "https://example.invalid/", Certs: tt.certs}
			result := New().Send(context.Background(), req)
			if result.Err == nil || !strings.Contains(result.Err.Error(), tt.want) {
				t.Errorf("Err = %v, want it to contain %q", result.Err, tt.want)
			}
		})
	}
}
