package interpolate

import (
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
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

func TestApplyDynamicVariables(t *testing.T) {
	got, missing, _ := Apply("{{$uuid}}|{{$timestamp}}|{{$isoTimestamp}}|{{$randomInt}}", nil)
	if len(missing) != 0 {
		t.Fatalf("missing = %v, want none", missing)
	}
	parts := strings.Split(got, "|")
	if len(parts) != 4 {
		t.Fatalf("got %q, want 4 parts", got)
	}
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(parts[0]) {
		t.Errorf("$uuid = %q, want a v4 UUID", parts[0])
	}
	if ts, err := strconv.ParseInt(parts[1], 10, 64); err != nil || time.Since(time.Unix(ts, 0)) > time.Minute {
		t.Errorf("$timestamp = %q, want current unix seconds", parts[1])
	}
	if ts, err := time.Parse(time.RFC3339, parts[2]); err != nil || time.Since(ts) > time.Minute || !strings.HasSuffix(parts[2], "Z") {
		t.Errorf("$isoTimestamp = %q, want current UTC RFC3339", parts[2])
	}
	if n, err := strconv.Atoi(parts[3]); err != nil || n < 0 || n > 1000 {
		t.Errorf("$randomInt = %q, want 0..1000", parts[3])
	}
}

func TestApplyDynamicVariableFreshPerOccurrence(t *testing.T) {
	got, _, _ := Apply("{{$uuid}} {{$uuid}}", nil)
	a, b, _ := strings.Cut(got, " ")
	if a == b {
		t.Errorf("both occurrences gave %q, want distinct values", a)
	}
}

func TestApplyUnsupportedDynamicVariableIsReportedMissing(t *testing.T) {
	got, missing, _ := Apply("id={{$nope}}", map[string]string{"nope": "declared"})
	if want := "id={{$nope}}"; got != want {
		t.Errorf("got %q, want %q (must not fall back to the plain variable)", got, want)
	}
	if want := []string{"$nope"}; !reflect.DeepEqual(missing, want) {
		t.Errorf("missing = %v, want %v", missing, want)
	}
}

func TestApplyURLEscapesDynamicQueryValue(t *testing.T) {
	got, _, _ := ApplyURL("http://localhost:8080/get?at={{$isoTimestamp}}", nil)
	if strings.Contains(got, ":") && strings.Contains(strings.SplitN(got, "?", 2)[1], ":") {
		t.Errorf("got %q, want the timestamp's colons percent-encoded in the query value", got)
	}
}

func TestDynamicVariableNames(t *testing.T) {
	want := []string{"$isoTimestamp", "$randomInt", "$timestamp", "$uuid"}
	if got := DynamicVariableNames(); !reflect.DeepEqual(got, want) {
		t.Errorf("DynamicVariableNames() = %v, want %v", got, want)
	}
}
