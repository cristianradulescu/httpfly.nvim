package client

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptrace"
	"strings"
	"sync"
	"time"
)

// tracer records what happens while a request is in flight: one line per
// event (Result.Trace, shown with -v), plus the phase it last reached and
// the proxy it went through, so a failure can say where it happened. Go's
// own errors often don't -- a protocol mismatch with a proxy is just
// "unexpected EOF".
//
// Its callbacks can run on the transport's dialing goroutines, hence mu.
type tracer struct {
	start time.Time

	mu    sync.Mutex
	lines []string
	phase string
	proxy string
}

func newTracer() *tracer {
	return &tracer{start: time.Now(), phase: "preparing the request"}
}

// event appends a trace line and, when phase is non-empty, records it as
// the phase the request is now in.
func (t *tracer) event(phase, format string, args ...any) {
	t.mu.Lock()
	defer t.mu.Unlock()
	elapsed := float64(time.Since(t.start).Microseconds()) / 1000
	t.lines = append(t.lines, fmt.Sprintf("%8.1fms  %s", elapsed, fmt.Sprintf(format, args...)))
	if phase != "" {
		t.phase = phase
	}
}

func (t *tracer) setProxy(proxy string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.proxy = proxy
}

// fail records err as the trace's last line and returns it annotated with
// the phase it happened in and the proxy in use, if any.
func (t *tracer) fail(err error) error {
	t.event("", "error: %v", err)
	t.mu.Lock()
	defer t.mu.Unlock()
	where := "while " + t.phase
	if t.proxy != "" {
		where += ", via proxy " + t.proxy
	}
	return fmt.Errorf("%w (%s)", err, where)
}

func (t *tracer) trace() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string(nil), t.lines...)
}

func (t *tracer) clientTrace() *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		GetConn: func(hostPort string) {
			t.event("getting a connection for "+hostPort, "getting a connection for %s", hostPort)
		},
		DNSStart: func(info httptrace.DNSStartInfo) {
			t.event("resolving "+info.Host, "resolving %s", info.Host)
		},
		DNSDone: func(info httptrace.DNSDoneInfo) {
			if info.Err != nil {
				t.event("", "DNS lookup failed: %v", info.Err)
				return
			}
			addrs := make([]string, len(info.Addrs))
			for i, a := range info.Addrs {
				addrs[i] = a.String()
			}
			t.event("", "resolved to %s", strings.Join(addrs, ", "))
		},
		ConnectStart: func(network, addr string) {
			t.event("connecting to "+addr, "connecting to %s (%s)", addr, network)
		},
		ConnectDone: func(network, addr string, err error) {
			if err != nil {
				t.event("", "connecting to %s failed: %v", addr, err)
				return
			}
			t.event("", "connected to %s", addr)
		},
		TLSHandshakeStart: func() {
			t.event("during the TLS handshake", "TLS handshake started")
		},
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			if err != nil {
				t.event("", "TLS handshake failed: %v", err)
				return
			}
			t.event("", "TLS handshake done (%s, %s)", tls.VersionName(state.Version), tls.CipherSuiteName(state.CipherSuite))
		},
		GotConn: func(info httptrace.GotConnInfo) {
			if info.Reused {
				t.event("sending the request", "reusing connection to %s", info.Conn.RemoteAddr())
				return
			}
			t.event("sending the request", "got connection to %s", info.Conn.RemoteAddr())
		},
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			if info.Err != nil {
				t.event("", "writing the request failed: %v", info.Err)
				return
			}
			t.event("waiting for the response", "request sent, waiting for the response")
		},
		GotFirstResponseByte: func() {
			t.event("reading the response headers", "first response byte received")
		},
	}
}

// tracingTransport logs each round trip -- so every redirect hop shows up
// -- and the proxy it goes through.
type tracingTransport struct {
	base   http.RoundTripper
	t      *tracer
	envVar bool // the proxy, if any, comes from HTTP_PROXY etc., not @proxy
}

func (tt *tracingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	tt.t.event("", "%s %s", req.Method, req.URL.Redacted())
	if tr, ok := tt.base.(*http.Transport); ok && tr.Proxy != nil {
		if u, err := tr.Proxy(req); err == nil && u != nil {
			proxy := u.Redacted()
			tt.t.setProxy(proxy)
			if tt.envVar {
				tt.t.event("", "via proxy %s (from the environment)", proxy)
			} else {
				tt.t.event("", "via proxy %s", proxy)
			}
		}
	}

	resp, err := tt.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	tt.t.event("reading the response body", "response: %s %s", resp.Proto, resp.Status)
	return resp, nil
}
