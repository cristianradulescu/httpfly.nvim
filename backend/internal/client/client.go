// Package client sends httpfile.Request values over HTTP.
package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cristianradulescu/httpfly.nvim/backend/internal/httpfile"
)

// DefaultTimeout is the per-request timeout New applies (see Client.HTTP)
// when the caller doesn't override it: covering connection, request, and
// reading the whole response body.
const DefaultTimeout = 30 * time.Second

// Result is the outcome of sending a single request. A non-nil Err means the
// request couldn't be sent or the response couldn't be read -- it does not
// mean the server returned a non-2xx status, which is a normal Result with
// Err == nil.
type Result struct {
	Request    httpfile.Request
	StatusCode int
	Status     string
	Headers    http.Header
	Body       []byte
	Duration   time.Duration
	Err        error
	// FinalURL is the URL the response actually came from, after following
	// any redirects. It equals Request.URL when there were none.
	FinalURL string
	// TLS is the connection's TLS state, or nil for a plain HTTP request.
	TLS *tls.ConnectionState
	// Trace is a timestamped log of what happened while sending: DNS,
	// connecting, proxy, TLS handshake, each redirect hop, and the error
	// if any (see tracer).
	Trace []string
}

// Client sends httpfile.Request values over HTTP.
type Client struct {
	HTTP *http.Client

	mu      sync.Mutex
	clients map[transportKey]*http.Client
}

// transportKey identifies the non-default transport settings a request
// needs; requests with the same key share one *http.Client.
type transportKey struct {
	proxy string
	certs httpfile.Certificates
}

// New returns a Client with DefaultTimeout. Set HTTP.Timeout before the
// first Send to use a different one; a proxy/certificate client created
// later copies whatever HTTP.Timeout is at that point.
func New() *Client {
	return &Client{
		HTTP:    &http.Client{Timeout: DefaultTimeout},
		clients: make(map[transportKey]*http.Client),
	}
}

// Send performs req and returns its outcome. If req.Proxy is set, the
// request is sent through that proxy instead of connecting directly; if
// req.Certs is set, its certificate files are loaded into the TLS setup.
func (c *Client) Send(ctx context.Context, req httpfile.Request) Result {
	start := time.Now()

	httpClient, err := c.clientFor(transportKey{proxy: req.Proxy, certs: req.Certs})
	if err != nil {
		return Result{Request: req, Err: err}
	}

	var body io.Reader
	if req.Body != "" {
		body = strings.NewReader(req.Body)
	}

	tr := newTracer()
	ctx = httptrace.WithClientTrace(ctx, tr.clientTrace())
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, body)
	if err != nil {
		return Result{Request: req, Err: err}
	}
	for _, h := range req.Headers {
		httpReq.Header.Add(h.Name, h.Value)
	}

	// A per-request copy, so the cached client's connection pool is shared
	// but the tracing transport isn't.
	traced := *httpClient
	base := httpClient.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	traced.Transport = &tracingTransport{base: base, t: tr, envVar: req.Proxy == ""}

	resp, err := traced.Do(httpReq)
	if err != nil {
		err = tr.fail(err)
		return Result{Request: req, Duration: time.Since(start), Err: err, Trace: tr.trace()}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	duration := time.Since(start)
	if err != nil {
		err = tr.fail(err)
		return Result{Request: req, Duration: duration, Err: err, Trace: tr.trace()}
	}
	tr.event("", "response body read (%d bytes)", len(respBody))

	finalURL := req.URL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}

	return Result{
		Request:    req,
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Headers:    resp.Header,
		Body:       respBody,
		Duration:   duration,
		FinalURL:   finalURL,
		TLS:        resp.TLS,
		Trace:      tr.trace(),
	}
}

// clientFor returns the *http.Client to use for a request: the shared
// default client when key is zero, or one with its own transport (routed
// through key.proxy, presenting/trusting key.certs) otherwise. These are
// cached so requests with the same settings share a connection pool.
func (c *Client) clientFor(key transportKey) (*http.Client, error) {
	if key == (transportKey{}) {
		return c.HTTP, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if hc, ok := c.clients[key]; ok {
		return hc, nil
	}

	transport := &http.Transport{}
	if !key.certs.IsZero() {
		tlsConfig, err := loadTLSConfig(key.certs)
		if err != nil {
			return nil, err
		}
		// Same defaults (dial timeouts, HTTP/2, proxy from the environment)
		// as the default client, which a custom TLS config would otherwise
		// lose.
		transport = http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = tlsConfig
	}
	if key.proxy != "" {
		proxyURL, err := url.Parse(key.proxy)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	hc := &http.Client{Timeout: c.HTTP.Timeout, Transport: transport}
	c.clients[key] = hc
	return hc, nil
}

// loadTLSConfig builds a TLS config from certs' PEM files: CACertificate is
// added to the system root CAs, and ClientCertificate (with its key from
// ClientCertificateKey, or from the same file when that's unset) is
// presented to servers that ask for one.
func loadTLSConfig(certs httpfile.Certificates) (*tls.Config, error) {
	cfg := &tls.Config{}

	if certs.CACertificate != "" {
		pemData, err := os.ReadFile(certs.CACertificate)
		if err != nil {
			return nil, fmt.Errorf("read CA certificate: %w", err)
		}
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(pemData) {
			return nil, fmt.Errorf("CA certificate %q: no PEM certificates found", certs.CACertificate)
		}
		cfg.RootCAs = pool
	}

	if certs.ClientCertificate != "" {
		keyFile := certs.ClientCertificateKey
		if keyFile == "" {
			keyFile = certs.ClientCertificate
		}
		cert, err := tls.LoadX509KeyPair(certs.ClientCertificate, keyFile)
		if err != nil {
			return nil, fmt.Errorf("load client certificate: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}

	return cfg, nil
}
