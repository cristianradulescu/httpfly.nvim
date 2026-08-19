package cli

// Version is httpfly's version. It defaults to "dev" for local builds (go
// run, go build without flags) and is overridden at release-build time via:
//
//	go build -ldflags "-X github.com/cristianradulescu/httpfly/internal/cli.Version=X.Y.Z"
//
// (see the Makefile's build target).
var Version = "dev"
