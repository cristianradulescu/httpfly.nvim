package cli

import (
	"crypto/tls"
	"encoding/json"
	"io"

	"github.com/cristianradulescu/httpfly/internal/httpfile"
)

// jsonResult is the -json output shape for one request: the request as
// sent, the response as received, and either is present -- a transport
// failure (DNS, connection refused, ...) has no response, just Error.
type jsonResult struct {
	Name        string        `json:"name"`
	Request     jsonRequest   `json:"request"`
	Response    *jsonResponse `json:"response,omitempty"`
	FinalURL    string        `json:"final_url,omitempty"`
	Error       string        `json:"error,omitempty"`
	ScriptError string        `json:"script_error,omitempty"`
	DurationMs  int64         `json:"duration_ms"`
}

type jsonRequest struct {
	Method  string              `json:"method"`
	URL     string              `json:"url"`
	Proto   string              `json:"proto"`
	Headers map[string][]string `json:"headers"`
	Body    string              `json:"body,omitempty"`
}

type jsonResponse struct {
	StatusCode   int                 `json:"status_code"`
	Headers      map[string][]string `json:"headers"`
	Body         string              `json:"body"`
	DownloadPath string              `json:"download_path,omitempty"`
	TLS          *jsonTLS            `json:"tls,omitempty"`
}

type jsonTLS struct {
	Version            string            `json:"version"`
	CipherSuite        string            `json:"cipher_suite"`
	NegotiatedProtocol string            `json:"negotiated_protocol,omitempty"`
	PeerCertificates   []jsonCertificate `json:"peer_certificates,omitempty"`
}

type jsonCertificate struct {
	Subject  string `json:"subject"`
	Issuer   string `json:"issuer"`
	NotAfter string `json:"not_after"`
}

func toJSONResult(o requestOutcome, verbose bool) jsonResult {
	r := o.Result
	out := jsonResult{
		Name: r.Request.Name,
		Request: jsonRequest{
			Method:  r.Request.Method,
			URL:     r.Request.URL,
			Proto:   r.Request.Proto,
			Headers: headersToMap(r.Request.Headers),
			Body:    r.Request.Body,
		},
		DurationMs: r.Duration.Milliseconds(),
	}

	if r.Err != nil {
		out.Error = r.Err.Error()
		return out
	}

	if r.FinalURL != "" && r.FinalURL != r.Request.URL {
		out.FinalURL = r.FinalURL
	}

	out.Response = &jsonResponse{
		StatusCode: r.StatusCode,
		Headers:    map[string][]string(r.Headers),
		Body:       string(r.Body),
	}
	if o.DownloadPath != "" {
		out.Response.Body = ""
		out.Response.DownloadPath = o.DownloadPath
	}
	if verbose {
		out.Response.TLS = toJSONTLS(r.TLS)
	}
	if o.ScriptErr != nil {
		out.ScriptError = o.ScriptErr.Error()
	}
	return out
}

func toJSONTLS(state *tls.ConnectionState) *jsonTLS {
	if state == nil {
		return nil
	}

	out := &jsonTLS{
		Version:            tls.VersionName(state.Version),
		CipherSuite:        tls.CipherSuiteName(state.CipherSuite),
		NegotiatedProtocol: state.NegotiatedProtocol,
	}
	for _, cert := range state.PeerCertificates {
		out.PeerCertificates = append(out.PeerCertificates, jsonCertificate{
			Subject:  cert.Subject.String(),
			Issuer:   cert.Issuer.String(),
			NotAfter: cert.NotAfter.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	return out
}

func headersToMap(headers []httpfile.Header) map[string][]string {
	m := make(map[string][]string, len(headers))
	for _, h := range headers {
		m[h.Name] = append(m[h.Name], h.Value)
	}
	return m
}

func printJSONResults(w io.Writer, outcomes []requestOutcome, verbose bool) error {
	out := make([]jsonResult, len(outcomes))
	for i, o := range outcomes {
		out[i] = toJSONResult(o, verbose)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
