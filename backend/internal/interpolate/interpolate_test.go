package interpolate

import (
	"reflect"
	"testing"
)

func TestApplyResolvesKnownVariables(t *testing.T) {
	got, missing, _ := Apply("{{scheme}}://{{host}}/get", map[string]string{"scheme": "http", "host": "localhost:8080"})
	if got != "http://localhost:8080/get" {
		t.Errorf("got %q", got)
	}
	if len(missing) != 0 {
		t.Errorf("missing = %v, want none", missing)
	}
}

func TestApplyLeavesUnknownPlaceholdersAndReportsThem(t *testing.T) {
	got, missing, _ := Apply("{{host}}/get?id={{id}}&again={{id}}", map[string]string{"host": "localhost"})
	if got != "localhost/get?id={{id}}&again={{id}}" {
		t.Errorf("got %q", got)
	}
	if want := []string{"id"}; !reflect.DeepEqual(missing, want) {
		t.Errorf("missing = %v, want %v (deduplicated)", missing, want)
	}
}

func TestApplyNoPlaceholders(t *testing.T) {
	got, missing, _ := Apply("plain text", nil)
	if got != "plain text" || len(missing) != 0 {
		t.Errorf("got %q, missing %v", got, missing)
	}
}

func TestApplyURLEscapesQueryValues(t *testing.T) {
	got, missing, _ := ApplyURL("http://localhost:8080/get?greeting={{greeting}}", map[string]string{"greeting": "Hello again"})
	if want := "http://localhost:8080/get?greeting=Hello+again"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if len(missing) != 0 {
		t.Errorf("missing = %v, want none", missing)
	}
}

func TestApplyURLLeavesHostPrefixVariableRaw(t *testing.T) {
	// {{host}} commonly holds a whole "scheme://host:port" prefix -- it must
	// not get percent-encoded (that would mangle the "://" into "%3A%2F%2F").
	got, missing, _ := ApplyURL("{{host}}/get?greeting={{greeting}}", map[string]string{
		"host": "http://localhost:8080", "greeting": "hi",
	})
	if want := "http://localhost:8080/get?greeting=hi"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if len(missing) != 0 {
		t.Errorf("missing = %v, want none", missing)
	}
}

func TestApplyURLLeavesPathSegmentVariableRaw(t *testing.T) {
	got, _, _ := ApplyURL("http://localhost:8080/users/{{id}}", map[string]string{"id": "abc-123"})
	if want := "http://localhost:8080/users/abc-123"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestApplyURLEscapesMultipleQueryParams(t *testing.T) {
	got, _, _ := ApplyURL("http://localhost:8080/get?a={{a}}&b={{b}}", map[string]string{"a": "x y", "b": "1&2"})
	if want := "http://localhost:8080/get?a=x+y&b=1%262"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestApplyURLLeavesValuelessQueryFlagRaw(t *testing.T) {
	got, _, _ := ApplyURL("http://localhost:8080/get?{{flag}}", map[string]string{"flag": "debug"})
	if want := "http://localhost:8080/get?debug"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestApplyURLNoQueryString(t *testing.T) {
	got, missing, _ := ApplyURL("{{host}}/get", map[string]string{"host": "http://localhost:8080"})
	if want := "http://localhost:8080/get"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if len(missing) != 0 {
		t.Errorf("missing = %v, want none", missing)
	}
}

func TestApplyURLMissingQueryValueLeftUntouchedAndReported(t *testing.T) {
	got, missing, _ := ApplyURL("http://localhost:8080/get?token={{token}}", nil)
	if want := "http://localhost:8080/get?token={{token}}"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if want := []string{"token"}; !reflect.DeepEqual(missing, want) {
		t.Errorf("missing = %v, want %v", missing, want)
	}
}

func TestApplyURLDedupesMissingAcrossBaseAndQuery(t *testing.T) {
	_, missing, _ := ApplyURL("http://localhost:8080/{{x}}?a={{x}}", nil)
	if want := []string{"x"}; !reflect.DeepEqual(missing, want) {
		t.Errorf("missing = %v, want %v (deduplicated across base and query)", missing, want)
	}
}

func TestApplyExpandsVariablesReferencingVariables(t *testing.T) {
	vars := map[string]string{
		"scheme": "http",
		"host":   "{{scheme}}://localhost:8080",
		"base":   "{{host}}/api",
	}
	got, missing, cycles := Apply("{{base}}/users", vars)
	if want := "http://localhost:8080/api/users"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if len(missing) != 0 || len(cycles) != 0 {
		t.Errorf("missing = %v, cycles = %v, want none", missing, cycles)
	}
}

func TestApplyReportsMissingVariableInsideAValue(t *testing.T) {
	got, missing, _ := Apply("{{base}}/users", map[string]string{"base": "{{host}}/api"})
	if want := "{{host}}/api/users"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if want := []string{"host"}; !reflect.DeepEqual(missing, want) {
		t.Errorf("missing = %v, want %v", missing, want)
	}
}

func TestApplyDetectsCycles(t *testing.T) {
	vars := map[string]string{"a": "{{b}}", "b": "{{a}}", "self": "{{self}}"}
	got, missing, cycles := Apply("{{a}} {{self}}", vars)
	if want := "{{a}} {{self}}"; got != want {
		t.Errorf("got %q, want placeholders left untouched %q", got, want)
	}
	if len(missing) != 0 {
		t.Errorf("missing = %v, want none (a cycle is not an undefined variable)", missing)
	}
	if want := []string{"a -> b -> a", "self -> self"}; !reflect.DeepEqual(cycles, want) {
		t.Errorf("cycles = %v, want %v", cycles, want)
	}
}

func TestApplyURLEscapesNestedValueOnce(t *testing.T) {
	vars := map[string]string{"greeting": "{{word}} again", "word": "Hello"}
	got, _, _ := ApplyURL("http://localhost:8080/get?greeting={{greeting}}", vars)
	if want := "http://localhost:8080/get?greeting=Hello+again"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
