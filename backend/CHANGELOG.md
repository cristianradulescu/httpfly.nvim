# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- `run -download F` saves a response body to file `F` instead of printing
  it, for binary responses (images, PDFs, archives, ...) that would
  otherwise dump raw bytes to the terminal. Requires selecting exactly one
  request; `F`'s parent directory must already exist (matching `curl -o`,
  which also silently overwrites an existing file at that path). Body is
  saved for any received response regardless of status code, but not on a
  transport failure. Compatible with `-json` (empties `response.body`,
  adds `response.download_path`); mutually exclusive with `-s`/`-silent`.

### Changed

- **Breaking:** `httpfly.env.json` and `.httpfly/state.json` are now
  resolved against the **current working directory** (wherever `httpfly`
  is launched from), not the directory containing the `.http` file being
  acted on. There is no fallback to the old file-relative lookup. This
  lets several `.http` files in different subdirectories (e.g. `v1/`,
  `v2/` of an API) share one environment file and one persisted
  `client.global` state, as long as `httpfly` is invoked from their common
  parent directory. If you relied on the old behavior — e.g. running
  `httpfly run some/dir/request.http` from an unrelated working directory
  — move or symlink `httpfly.env.json`/`.httpfly/` accordingly, or `cd`
  into the `.http` file's directory first.

## [0.1.2] - 2026-08-23

### Added

- `httpfly convert from-curl` — converts a bash-style curl command (e.g. a
  browser's "Copy as cURL" output) into a `.http` request block. Handles
  multi-line commands with `\` continuations, single/double-quoted
  arguments (including values with embedded quotes/colons/commas), both
  `curl <url>` and `curl --url <url>` forms, and maps `-H`/`-b`/`-u`/`-x`/
  `-d` onto headers/Cookie/Basic-auth/`@proxy`/body respectively. Reads
  from stdin by default, or a file with `-file`. Flags with no httpfly
  equivalent (`-k`, `-o`, file-based `-d @file`) are dropped with a
  warning on stderr, never polluting the generated file on stdout.
  Windows `cmd.exe`/PowerShell "Copy as cURL" variants aren't supported.
- `httpfly convert to-curl` — the reverse: converts one request from an
  `.http` file into a multi-line, bash-style curl command. `-name` picks
  the request (required if the file has more than one); `-env` resolves
  variables the same way `run`/`validate` do, but no script runs as part
  of the conversion, and a variable still undefined afterward is a hard
  error rather than a warning, since there's no later chance to fill it
  in the way `run` has.

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
