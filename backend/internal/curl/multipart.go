package curl

import (
	"fmt"
	"path"
	"strings"
)

// multipartBoundary is the fixed boundary httpfly generates for a
// converted multipart/form-data body -- same idea as the hand-written
// "WebAppBoundary" in doc/examples/5_forms.http, just generated instead of
// chosen by hand.
const multipartBoundary = "----HttpflyBoundary"

// formPart is one field of a multipart/form-data body -- either a plain
// text value, or a reference to a file's content (an upload).
type formPart struct {
	Name string
	// Value is the field's literal text content; unset when IsFile.
	Value  string
	IsFile bool
	// FilePath is the path curl's "@path" (or the .http body's own
	// "< path" reference) named -- never read here; the actual bytes are
	// only ever read at request-resolve time (internal/parser's
	// spliceFileReferences), so a converted file is referenced by path in
	// both directions, never inlined.
	FilePath    string
	Filename    string // the part's filename= attribute, if any
	ContentType string // the part's Content-Type header, if any
}

// parseFormField parses one "-F"/"--form" value, curl's own
// semicolon-separated grammar: "name=value" for a plain field, or
// "name=@path[;filename=X][;type=Y]" for a file upload (curl defaults
// filename to path's own base name unless overridden).
func parseFormField(raw string) (formPart, error) {
	name, rest, found := strings.Cut(raw, "=")
	if !found {
		return formPart{}, fmt.Errorf("expected \"name=value\", got %q", raw)
	}

	fields := strings.Split(rest, ";")
	part := formPart{Name: name}
	if filePath, ok := strings.CutPrefix(fields[0], "@"); ok {
		part.IsFile = true
		part.FilePath = filePath
		part.Filename = path.Base(filePath)
	} else {
		part.Value = fields[0]
	}

	for _, attr := range fields[1:] {
		key, value, ok := strings.Cut(attr, "=")
		if !ok {
			continue
		}
		switch key {
		case "filename":
			part.Filename = value
		case "type":
			part.ContentType = value
		}
	}
	return part, nil
}

// formatFormFlagValue is parseFormField's inverse: renders one formPart
// back into curl's own "-F" value grammar. The "filename=" attribute is
// only included when it differs from what curl would default to (the
// path's own base name), keeping the round-tripped command as close to
// what a human would actually type as possible.
func formatFormFlagValue(p formPart) string {
	if !p.IsFile {
		return p.Name + "=" + p.Value
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s=@%s", p.Name, p.FilePath)
	if p.Filename != "" && p.Filename != path.Base(p.FilePath) {
		fmt.Fprintf(&b, ";filename=%s", p.Filename)
	}
	if p.ContentType != "" {
		fmt.Fprintf(&b, ";type=%s", p.ContentType)
	}
	return b.String()
}

// formatMultipartBody renders parts as literal ".http" body text bounded
// by boundary, matching the shape httpfly's own parser expects (and
// doc/examples/5_forms.http demonstrates by hand): "--boundary" markers,
// a Content-Disposition line per part, and a file part's content spliced
// in at send time via "< path" (see internal/parser's
// spliceFileReferences) rather than inlined here.
func formatMultipartBody(parts []formPart, boundary string) string {
	var b strings.Builder
	for _, p := range parts {
		fmt.Fprintf(&b, "--%s\n", boundary)
		if p.IsFile {
			fmt.Fprintf(&b, "Content-Disposition: form-data; name=%q; filename=%q\n", p.Name, p.Filename)
		} else {
			fmt.Fprintf(&b, "Content-Disposition: form-data; name=%q\n", p.Name)
		}
		if p.ContentType != "" {
			fmt.Fprintf(&b, "Content-Type: %s\n", p.ContentType)
		}
		b.WriteString("\n")
		if p.IsFile {
			fmt.Fprintf(&b, "< %s\n", p.FilePath)
		} else {
			b.WriteString(p.Value)
			b.WriteString("\n")
		}
	}
	fmt.Fprintf(&b, "--%s--", boundary)
	return b.String()
}

// parseMultipartBody is formatMultipartBody's inverse: given a request's
// Content-Type header value and its still-templated RawBody, it reports
// whether the body is a multipart/form-data body in httpfly's own shape
// and, if so, the parts it contains -- used by ToCurl to regenerate "-F"
// flags instead of dumping the whole (possibly huge, possibly binary)
// resolved body as a single "--data-raw" argument.
func parseMultipartBody(contentType, rawBody string) ([]formPart, bool) {
	boundary, ok := multipartBoundaryFromContentType(contentType)
	if !ok {
		return nil, false
	}

	delimiter := "--" + boundary
	segments := strings.Split(rawBody, delimiter)
	// segments[0] is whatever precedes the first boundary marker (normally
	// empty); the last is the closing "--" plus trailing text.
	if len(segments) < 3 {
		return nil, false
	}

	var parts []formPart
	for _, seg := range segments[1 : len(segments)-1] {
		seg = strings.Trim(seg, "\n")
		headerText, content, found := strings.Cut(seg, "\n\n")
		if !found {
			return nil, false
		}

		part := formPart{}
		var haveName bool
		for _, line := range strings.Split(headerText, "\n") {
			name, value, found := strings.Cut(line, ":")
			if !found {
				return nil, false
			}
			value = strings.TrimSpace(value)
			switch strings.ToLower(strings.TrimSpace(name)) {
			case "content-disposition":
				part.Name, part.Filename, haveName = parseContentDisposition(value)
			case "content-type":
				part.ContentType = value
			}
		}
		if !haveName {
			return nil, false
		}

		content = strings.TrimSuffix(content, "\n")
		if filePath, ok := strings.CutPrefix(content, "< "); ok && !strings.Contains(content, "\n") {
			part.IsFile = true
			part.FilePath = filePath
		} else {
			part.Value = content
		}
		parts = append(parts, part)
	}
	return parts, true
}

// multipartBoundaryFromContentType extracts the "boundary=" parameter from
// a "multipart/form-data; boundary=X" Content-Type value.
func multipartBoundaryFromContentType(contentType string) (string, bool) {
	mediaType, params, _ := strings.Cut(contentType, ";")
	if !strings.EqualFold(strings.TrimSpace(mediaType), "multipart/form-data") {
		return "", false
	}
	for _, param := range strings.Split(params, ";") {
		key, value, found := strings.Cut(param, "=")
		if !found || strings.ToLower(strings.TrimSpace(key)) != "boundary" {
			continue
		}
		return strings.Trim(strings.TrimSpace(value), `"`), true
	}
	return "", false
}

// parseContentDisposition extracts name and filename from a
// "form-data; name=\"x\"; filename=\"y\"" Content-Disposition value.
func parseContentDisposition(value string) (name, filename string, ok bool) {
	for _, field := range strings.Split(value, ";") {
		key, val, found := strings.Cut(strings.TrimSpace(field), "=")
		if !found {
			continue
		}
		val = strings.Trim(strings.TrimSpace(val), `"`)
		switch strings.TrimSpace(key) {
		case "name":
			name, ok = val, true
		case "filename":
			filename = val
		}
	}
	return name, filename, ok
}
