# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

httpfly is a CLI HTTP client that runs `.http` files (REST Client / IntelliJ HTTP file syntax): plain-text request definitions with `###`-separated blocks, `# @key value` metadata, `{{var}}` variable interpolation, and (planned, not yet implemented) pre-/post-request scripting.

## Commands

```sh
make build   # go build -o bin/httpfly ./cmd/httpfly
make test    # go test ./...
make fmt     # gofmt -l -w .
make vet     # go vet ./...
make lint    # golangci-lint run (see .golangci.yml; must be installed separately)
make check   # fmt + vet + test -- run this before considering work done
```

Single test: `go test ./internal/parser/ -run TestAnalyzeResolvesFileScopedVariables -v`

Try the CLI against real requests without hitting the network:

```sh
make httpbin-up                                          # starts a local httpbin container on :8080
go run ./cmd/httpfly run doc/examples/1_basic.http
go run ./cmd/httpfly run -name Post doc/examples/1_basic.http
go run ./cmd/httpfly validate doc/examples/invalid.http   # -name X also works here
make httpbin-down
```

`internal/client`'s tests use `httptest.Server` and don't need httpbin; `doc/examples/*.http` files are meant to be run against it manually or read as parser fixtures (`parser_test.go` reads `1_basic.http` directly).

## Architecture

Pipeline: `parser.Parse` (text → validated `httpfile.File`) → `client.Send` (one request → one `Result`), wired together by `internal/cli`.

- **`internal/httpfile`** — plain domain types (`File`, `Request`, `Header`). No behavior; every other package depends on it.
- **`internal/parser`** — turns `.http` text into `httpfile.File`.
  - `parser.go`: splits the file on `###` lines into blocks (the segment *before* the first `###`, or the whole file if there's no `###` at all, is still block 0 — it carries no request but may declare file-scoped variables). Also handles `# @key value` metadata lines and `@name = value` variable declarations.
  - `validate.go`: the actual per-block parsing/validation logic (`validateBlock` and friends), producing a `Result` of `BlockResult{Request, Issues}` — a *lenient* pass that collects every `Issue` (`SeverityWarning`/`SeverityError`) instead of stopping at the first problem. `Parse()` is a thin wrapper over `Analyze()` that aggregates all `SeverityError` issues into one returned `error`; `Analyze()` itself only errors on unrecoverable I/O.
  - `@name` is **mandatory** on every request block (an empty/missing one is a `SeverityError`) — it's the sole identifier used for `-name` scoping in the CLI, so don't relax this without updating `internal/cli/select.go` and `filterResult`.
  - Unknown metadata keys, non-standard HTTP methods, unrecognized proto strings, and unresolved `{{var}}` placeholders are warnings (file is usable, just relying on something httpfly doesn't fully act on); malformed request/header lines and `< {%`/`> {%` scripting blocks (scripting isn't implemented yet) are errors.
  - Variable interpolation happens *during* validation, not as a separate pass: `collectVariables` walks every block first to build the file-wide `map[string]string` (later declarations win), then each block's URL/headers/body are resolved against it via `internal/interpolate`. A resolved header still gets kept even if interpolation left an unresolved placeholder in it — only a genuinely malformed header *line* drops the header (see `hasError` in `validate.go`; this was a real bug once, don't reintroduce it).
- **`internal/interpolate`** — just `Apply(s string, vars map[string]string) (result string, missing []string)`. Deliberately source-agnostic: it doesn't know or care whether `vars` came from a file declaration, an environment, or a script — that's on purpose, so environments/scripting can plug into it later without changing this package.
- **`internal/client`** — `Client.Send(ctx, httpfile.Request) Result` over `net/http`. A non-nil `Result.Err` means the request couldn't be sent/read (transport failure); a 4xx/5xx response is a normal `Result` with `Err == nil` — the CLI's exit code reflects only transport failures, not HTTP error statuses.
- **`internal/cli`** — argument parsing and command wiring (stdlib `flag`, no Cobra — intentionally minimal for the current two-command surface).
  - `run.go`: parses, resolves `-name` via `selectRequests`, sends each request, prints a report (request line/headers/body, then status/duration/headers/body).
  - `validate.go`: parses via `Analyze` (not `Parse`, since it wants warnings too), optionally narrows via `filterResult`, prints one section per block plus an error/warning summary count.
  - `select.go`: `selectRequests` — shared `-name` lookup logic for `run`.

## Design decisions worth knowing before changing things

- **Host header**: deliberately *not* supported. A relative-URL-plus-`Host`-header convenience was built, tested end-to-end against httpbin, and then reverted at the user's request — `.http` files must specify a fully absolute URL. Don't reintroduce this without being asked.
- **Variables before environments**: file-scoped `@name = value` interpolation was built first; environment files (`.env`-style, selected e.g. via `--env`) are intentionally deferred, feeding into the same `interpolate.Apply` machinery later rather than requiring a redesign.
- **Scripting is unimplemented but recognized**: `< {%`/`> {%` blocks and the `@lang` metadata key are detected and reported (as an error / a warning respectively) specifically so `validate` gives a clear signal rather than a confusing generic parse failure.
- **doc/examples/ numbering**: `1_basic.http` and `2_variables.http` are a deliberate walkthrough sequence (each file's header comment points to the next); `3_scripting.http` is reserved for when scripting lands. `doc/examples/invalid.http` is a separate, unnumbered fixture — one deliberately broken block per validation rule, used to exercise `validate`, not meant to be run.
