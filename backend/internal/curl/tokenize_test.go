package curl

import (
	"reflect"
	"testing"
)

func TestTokenizeSingleLine(t *testing.T) {
	got, err := Tokenize(`curl 'https://example.com/get' -H 'Accept: application/json'`)
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	want := []string{"curl", "https://example.com/get", "-H", "Accept: application/json"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestTokenizeBashLineContinuation(t *testing.T) {
	got, err := Tokenize("curl 'https://example.com/get' \\\n  -H 'Accept: application/json' \\\n  -H 'Accept-Language: en'\n")
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	want := []string{"curl", "https://example.com/get", "-H", "Accept: application/json", "-H", "Accept-Language: en"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestTokenizeEmbeddedDoubleQuotesInsideSingleQuotes(t *testing.T) {
	// A real Chrome example: If-None-Match values contain a raw ETag with
	// embedded double quotes -- single quotes make everything inside
	// literal, including those quotes.
	got, err := Tokenize(`curl 'https://x' -H 'if-none-match: W/"0c341038a74e06d043870e132158ed4e"'`)
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	want := []string{"curl", "https://x", "-H", `if-none-match: W/"0c341038a74e06d043870e132158ed4e"`}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestTokenizeEmbeddedEqualsSemicolonCommaInsideSingleQuotes(t *testing.T) {
	// Another real Chrome header: sec-ch-ua's value is full of "=", ";",
	// ",", and quotes -- none of it should be treated as a delimiter.
	got, err := Tokenize(`curl 'https://x' -H 'sec-ch-ua: "Not=A?Brand";v="99", "Google Chrome";v="151"'`)
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	want := []string{"curl", "https://x", "-H", `sec-ch-ua: "Not=A?Brand";v="99", "Google Chrome";v="151"`}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestTokenizeDoubleQuotedEscapes(t *testing.T) {
	got, err := Tokenize(`curl "https://x" -H "X-Test: a\"b\\c\$d"`)
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	want := []string{"curl", "https://x", "-H", `X-Test: a"b\c$d`}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestTokenizeAdjacentQuotesJoinIntoOneToken(t *testing.T) {
	got, err := Tokenize(`'foo'bar"baz"`)
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	want := []string{"foobarbaz"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestTokenizeUnterminatedSingleQuoteErrors(t *testing.T) {
	if _, err := Tokenize(`curl 'https://x`); err == nil {
		t.Fatal("expected an error for an unterminated single-quoted string")
	}
}

func TestTokenizeUnterminatedDoubleQuoteErrors(t *testing.T) {
	if _, err := Tokenize(`curl "https://x`); err == nil {
		t.Fatal("expected an error for an unterminated double-quoted string")
	}
}
