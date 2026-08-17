// Package client sends httpfile.Request values over HTTP.
package client

import (
	"context"
	"io"
	"net/http"
	"strings"
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
}

// Client sends httpfile.Request values over HTTP.
type Client struct {
	HTTP *http.Client
}

// New returns a Client with a default timeout.
func New() *Client {
	return &Client{HTTP: &http.Client{Timeout: defaultTimeout}}
}

// Send performs req and returns its outcome.
func (c *Client) Send(ctx context.Context, req httpfile.Request) Result {
	start := time.Now()

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

	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return Result{Request: req, Duration: time.Since(start), Err: err}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	duration := time.Since(start)
	if err != nil {
		return Result{Request: req, Duration: duration, Err: err}
	}

	return Result{
		Request:    req,
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Headers:    resp.Header,
		Body:       respBody,
		Duration:   duration,
	}
}
