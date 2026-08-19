// Package client sends httpfile.Request values over HTTP.
package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/cristianradulescu/httpfly/internal/httpfile"
)

const defaultTimeout = 30 * time.Second

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
}

// Client sends httpfile.Request values over HTTP.
type Client struct {
	HTTP *http.Client

	mu           sync.Mutex
	proxyClients map[string]*http.Client // keyed by httpfile.Request.Proxy
}

// New returns a Client with a default timeout.
func New() *Client {
	return &Client{
		HTTP:         &http.Client{Timeout: defaultTimeout},
		proxyClients: make(map[string]*http.Client),
	}
}

// Send performs req and returns its outcome. If req.Proxy is set, the
// request is sent through that proxy instead of connecting directly.
func (c *Client) Send(ctx context.Context, req httpfile.Request) Result {
	start := time.Now()

	httpClient, err := c.clientFor(req.Proxy)
	if err != nil {
		return Result{Request: req, Err: err}
	}

	var body io.Reader
	if req.Body != "" {
		body = strings.NewReader(req.Body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, body)
	if err != nil {
		return Result{Request: req, Err: err}
	}
	for _, h := range req.Headers {
		httpReq.Header.Add(h.Name, h.Value)
	}

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return Result{Request: req, Duration: time.Since(start), Err: err}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	duration := time.Since(start)
	if err != nil {
		return Result{Request: req, Duration: duration, Err: err}
	}

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
	}
}

// clientFor returns the *http.Client to use for a request: the shared
// direct-connection client when proxy is "", or a client routed through
// that proxy otherwise. Proxy clients are cached so requests sharing a
// proxy also share its connection pool.
func (c *Client) clientFor(proxy string) (*http.Client, error) {
	if proxy == "" {
		return c.HTTP, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if hc, ok := c.proxyClients[proxy]; ok {
		return hc, nil
	}

	proxyURL, err := url.Parse(proxy)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL: %w", err)
	}
	hc := &http.Client{
		Timeout:   c.HTTP.Timeout,
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
	}
	c.proxyClients[proxy] = hc
	return hc, nil
}
