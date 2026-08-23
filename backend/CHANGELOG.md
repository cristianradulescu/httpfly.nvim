# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

## [0.1.1] - 2026-08-23

### Added

- Pre-/post-request scripting: `< {% ... %}` (before a request is sent)
  and `> {% ... %}` (after its response arrives) run as Lua, with
  `client.global:get/set`, `response.status`/`headers`/`body` (post-request
  only), and file-read/command-exec/JSON via `require("ioutil"/"cmd"/"json"/"filepath")`
  ([gopher-lua-libs](https://github.com/vadv/gopher-lua-libs)).
- `client.global` persists to `.httpfly/state.json` (scoped per directory
  and per `-env` environment), so a value set by one request's
  post-request script — e.g. a token from a login request — is available
  to a later request, whether that's a later request in the *same*
  `httpfly run` or a separate, later invocation.
- Each request re-resolves its `{{variable}}` placeholders immediately
  before it's sent, using whatever `client.global` currently holds — so
  scripting works the same way whether you run a whole file at once or one
  request at a time. If a variable is still undefined at that point, `run`
  fails that request loudly rather than silently sending a literal
  `{{name}}`.
- `httpfly validate` compiles (but never runs) a script's Lua source, so
  syntax errors are caught with none of the side effects actually running
  it might have.
- `run -json` gains an optional `script_error` field for a post-request
  script that errored (the response itself is still included).

### Fixed

- `run -json` no longer prints a trailing `run: N of M request(s) failed`
  summary when one or more requests fail. That line went to stderr, not
  the JSON on stdout, but a consumer that merges the two streams (e.g. a
  container log, some CI runners) would see it appended right after the
  array, breaking a naive parse. The exit code still reflects failure, and
  each failed request's own `error`/`script_error` field already carries
  the detail the summary would have repeated.

## [0.1.0] - 2026-08-19

### Added

- `.http` file parsing: `###`-delimited request blocks, `# @key value`
  metadata, headers, and bodies.
- `httpfly validate` — reports every issue in a file (missing/empty
  `@name`, malformed request lines and headers, relative URLs,
  non-standard methods, unrecognized protocol versions, unknown metadata,
  undefined variables, ...) at `ERROR` or `WARN` severity, instead of
  stopping at the first problem.
- `httpfly run` — sends the requests a file defines and prints their
  responses.
- `{{variable}}` interpolation in the URL, headers, and body, with
  `key = value` declarations that are either global (declared in the
  file's prelude, before the first `###`) or local to one request block,
  local always overriding global.
- Automatic percent-encoding of a variable's value when it lands inside a
  query parameter's value, so values containing spaces or `&` don't
  corrupt the request; the URL's own scheme/host/path structure is left
  untouched.
- `@proxy` metadata to route a request (or, declared globally, every
  request) through an HTTP proxy.
- Environments: `httpfly.env.json` alongside a `.http` file, selected with
  `-env <name>`, supplying variables that override the file's own globals
  but not a request's local ones. Supports a `shared` section merged as
  defaults across every named environment.
- `-name` to restrict `run`/`validate` to a single request.
- `-s`/`-silent` to print only response bodies, like `curl -s`.
- `-json` for structured output: one object per request with the request
  sent, the response (or transport error) received, the final URL after
  any redirects, and duration.
- `-v`/`-verbose` to also report TLS connection details (version, cipher
  suite, ALPN protocol, peer certificate chain).
- `version` command and `-version`/`--version` flags.
