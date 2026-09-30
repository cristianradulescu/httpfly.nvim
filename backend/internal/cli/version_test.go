package cli

import "testing"

func TestEffectiveVersionPrefersLdflagsVersion(t *testing.T) {
	orig := Version
	defer func() { Version = orig }()

	Version = "1.2.3"
	if got := EffectiveVersion(); got != "1.2.3" {
		t.Errorf("EffectiveVersion() = %q, want %q (ldflags-set Version should win)", got, "1.2.3")
	}
}

func TestEffectiveVersionFallsBackWhenVersionIsDev(t *testing.T) {
	orig := Version
	defer func() { Version = orig }()

	Version = "dev"
	got := EffectiveVersion()
	if got == "" {
		t.Error("EffectiveVersion() = \"\", want a non-empty fallback")
	}
	// The test binary's own build info won't carry a real module version
	// (it's "(devel)" or empty under `go test`), so the fallback should
	// land back on "dev" here -- this just guards against EffectiveVersion
	// ever returning the literal build-info sentinel.
	if got == "(devel)" {
		t.Errorf("EffectiveVersion() = %q, want the \"(devel)\" sentinel filtered out", got)
	}
}
