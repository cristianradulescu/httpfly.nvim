package interpolate

import (
	"reflect"
	"testing"
)

func TestApplyResolvesKnownVariables(t *testing.T) {
	got, missing := Apply("{{scheme}}://{{host}}/get", map[string]string{"scheme": "http", "host": "localhost:8080"})
	if got != "http://localhost:8080/get" {
		t.Errorf("got %q", got)
	}
	if len(missing) != 0 {
		t.Errorf("missing = %v, want none", missing)
	}
}

func TestApplyLeavesUnknownPlaceholdersAndReportsThem(t *testing.T) {
	got, missing := Apply("{{host}}/get?id={{id}}&again={{id}}", map[string]string{"host": "localhost"})
	if got != "localhost/get?id={{id}}&again={{id}}" {
		t.Errorf("got %q", got)
	}
	if want := []string{"id"}; !reflect.DeepEqual(missing, want) {
		t.Errorf("missing = %v, want %v (deduplicated)", missing, want)
	}
}

func TestApplyNoPlaceholders(t *testing.T) {
	got, missing := Apply("plain text", nil)
	if got != "plain text" || len(missing) != 0 {
		t.Errorf("got %q, missing %v", got, missing)
	}
}
