package cli

import (
	"runtime/debug"
	"strings"
)

// Version is httpfly's version. It defaults to "dev" for local builds (go
// run, go build without flags) and is overridden at release-build time via:
//
//	go build -ldflags "-X github.com/cristianradulescu/httpfly/internal/cli.Version=X.Y.Z"
//
// (see the Makefile's build target). Callers that display a version should
// use EffectiveVersion instead, which also covers "go install".
var Version = "dev"

// EffectiveVersion returns Version if it was set via the ldflags above, or
// else falls back to the module version Go's toolchain embeds
// automatically in the binary when it's built with "go install
// .../cmd/httpfly@vX.Y.Z" (or any other module-aware build that doesn't go
// through the Makefile). Without this fallback, "go install" -- which
// never runs the Makefile and so never sets Version -- would always report
// "dev" even for a tagged release. The module version's leading "v" (e.g.
// "v0.3.0") is stripped so it matches the unprefixed X.Y.Z the Makefile's
// build produces, from the VERSION file -- and the tags this project's
// releases use are themselves "vX.Y.Z", so this is the same string either
// way.
func EffectiveVersion() string {
	if Version != "dev" {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return strings.TrimPrefix(v, "v")
		}
	}
	return Version
}
