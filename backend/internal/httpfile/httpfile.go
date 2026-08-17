// Package httpfile defines the domain types produced by parsing an .http file.
package httpfile

// File is a sequence of requests parsed from a single .http file.
type File struct {
	Variables map[string]string
	Requests  []Request
}

// Request is one ###-delimited block: a name, a method/URL/proto line,
// headers, and an optional body.
type Request struct {
	Name    string
	Method  string
	URL     string
	Proto   string
	Headers []Header
	Body    string
}

// Header is a single "Name: Value" line.
type Header struct {
	Name  string
	Value string
}
