// Package httpfile defines the domain types produced by parsing an .http file.
package httpfile

// File is a sequence of requests parsed from a single .http file.
type File struct {
	Variables map[string]string
	Requests  []Request
}

// Request is one ###-delimited block: a name, a method/URL/proto line,
// headers, and an optional body. URL/Headers/Body/Proxy are resolved
// (every "{{var}}" placeholder substituted) using whatever variables were
// visible when this Request was produced -- see RawURL etc. below for
// re-resolving against different (e.g. more current) variables.
type Request struct {
	Name    string
	Method  string
	URL     string
	Proto   string
	Headers []Header
	Body    string
	// Proxy is the absolute URL of an HTTP proxy to send this request
	// through, or "" to connect directly.
	Proxy string
	// Certs holds the TLS certificate files this request uses; the zero
	// value means the system defaults (no client certificate).
	Certs Certificates
	// Lang is the scripting language for PreScript/PostScript. Only "lua"
	// is currently supported; it's the default when unset.
	Lang string
	// PreScript is the source of a "< {% ... %}" block, run just before
	// this request is sent, or "" if there is none.
	PreScript string
	// PostScript is the source of a "> {% ... %}" block, run just after
	// this request's response is received, or "" if there is none.
	PostScript string

	// Variables holds this request's own local "@key = value" declarations
	// (not merged with any other scope), so a caller can recompute the
	// full variable set later (e.g. after a pre-request script changes a
	// persisted global) and still have this request's local overrides win.
	Variables map[string]string
	// RawURL, RawHeaders, RawBody, RawProxy, and RawCerts are this request's
	// still-templated ("{{var}}" not yet substituted) source, exactly as
	// written in the .http file. They're what a resolver re-interpolates
	// against fresh variables -- see the parser package's Resolve.
	RawURL     string
	RawHeaders []Header
	RawBody    string
	RawProxy   string
	RawCerts   Certificates
}

// Certificates are the PEM file paths for a request's TLS setup, named
// after JetBrains HTTP Client's SSLConfiguration (plus CACertificate,
// which it has no equivalent for). Each is "" when unset. Comparable, so
// it can key a cache of HTTP clients.
type Certificates struct {
	// ClientCertificate is the certificate presented to the server (mutual
	// TLS). It may also hold the private key, in which case
	// ClientCertificateKey can be left unset.
	ClientCertificate string
	// ClientCertificateKey is ClientCertificate's private key, when it's
	// stored in a separate file.
	ClientCertificateKey string
	// CACertificate is trusted for verifying the server, on top of the
	// system's root CAs.
	CACertificate string
}

// IsZero reports whether no certificate file is set.
func (c Certificates) IsZero() bool {
	return c == Certificates{}
}

// Header is a single "Name: Value" line.
type Header struct {
	Name  string
	Value string
}
