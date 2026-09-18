# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- `run -timeout D` sets the per-request timeout (default `30s`, `0` to
  disable) as a Go duration like `2m`, covering the connection, the
  request, and reading the whole response body — previously the 30s was
  hardcoded, so a large `-download` over a slow link had no way to
  succeed.
- Dynamic variables, matching JetBrains HTTP Client's names: `{{$uuid}}`,
  `{{$timestamp}}`, `{{$isoTimestamp}}` and `{{$randomInt}}` are generated
  fresh for every occurrence when a request is resolved. Any other
  `{{$name}}` is reported as an undefined variable; previously a
  `$`-prefixed placeholder was silently sent as literal text.
- A variable's value can now reference other variables
  (`@host = {{scheme}}://localhost:8080`), expanded recursively against
  the full variable set regardless of which tier (prelude, environment,
  persisted state, local) each one comes from — matching JetBrains HTTP
  Client. An undefined variable inside a value is reported like any other;
  a reference cycle is a validation `ERROR`.
- `#` comment lines are now allowed among a request's headers, so a header
  can be commented out and back in without moving it. A `# @key` metadata
  line in that position is an `ERROR` (metadata must precede the request
  line) rather than being silently ignored. `#` lines inside the body are
  still body content, unchanged.

### Changed

- A `multipart/*` body is now sent with CRLF line endings, as RFC 2046
  requires, regardless of how the `.http` file is saved — previously it
  went out byte-for-byte with bare LF, which strict servers reject.
  Bytes spliced in from a `< path` reference are untouched, and every
  other body is still sent exactly as written.
- A script's `print(...)` output now goes to stderr instead of stdout, so
  debugging output from a pre-/post-request script can never end up mixed
  into `run -json`'s stdout (which must stay a single clean JSON document
  for tools reading it).
- `run`/`validate` now print flag-parsing errors and their usage text to
  stderr, matching `convert`, for the same reason.

### Fixed

- The same `@name` used by more than one request block in a file is now a
  validation `ERROR` (reported on the second block). Previously `-name X`
  silently picked the first match, making the other block unreachable.
- A pre-request script failure was reported with a doubled
  `pre-request script: pre-request script: ...` prefix; it's now prefixed
  once. The plain-text `run` output likewise no longer prefixes a
  post-request script error twice.

## [0.3.1] - 2026-09-15

### Added

- File uploads: a body line `< path/to/file` splices that file's raw
  bytes into the request at send time — works as the whole body or
  inside one part of a hand-written multipart body. A relative path
  resolves against the current working directory, same as everything
  else httpfly reads from disk, and the path itself can use `{{var}}`
  interpolation. A file that can't be read is a validation warning
  (a pre-request script might still create it before the request is
  sent), escalated to a hard error by `run`/`convert to-curl` right
  before it's actually needed. See
  [File uploads](doc/http-file-format.md#file-uploads) and
  `doc/examples/8_file_upload.http`.
- `convert from-curl` understands curl's own file-upload flags:
  `-d`/`--data-binary @path` becomes a `< path` body reference instead
  of being dropped, and `-F`/`--form` builds a real
  `multipart/form-data` body — a plain `name=value` field becomes text,
  `name=@path[;filename=X][;type=Y]` becomes a file field. Mixing `-F`
  with any `-d`/`--data*` is dropped with a warning, matching curl's own
  restriction.
- `convert to-curl` is the reverse: a `< path` body becomes
  `--data-binary @path`, and a multipart body built from `< path`
  references becomes one `-F` flag per part — the file is referenced by
  path in the generated command rather than its contents inlined.
- New walkthrough examples: `doc/examples/5_forms.http`
  (url-encoded/multipart bodies), `6_shell_auth.http` (shelling out to
  `generate-token.sh` from a pre-request script), `7_save_response.http`
  (saving a response body via `ioutil.write_file`), and
  `8_file_upload.http` (the new `< path` syntax, paired with the
  `8_photo.png` fixture).

### Fixed

- `version`/`-version`/`--version` now reports the correct version when
  httpfly is installed via `go install .../cmd/httpfly@vX.Y.Z` (falls
  back to the module version Go's toolchain embeds automatically)
  instead of always printing `dev` outside of a `make build`.

## [0.3.0] - 2026-09-12

### Added

- A block's `###` separator line can carry the request's name directly
  (`### GetUsers`), equivalent to a bare `###` followed by
  `# @name GetUsers`. An explicit `# @name` line in the same block is
  still allowed as long as it agrees with the separator; a conflicting
  value is a validation error instead of one silently overwriting the
  other.
- Optional `http-client.private.env.json`, alongside `http-client.env.json`,
  for environment values you don't want committed (credentials, personal
  tokens) or a fully local-only environment. Its values override the
  public file's on a per-key basis, and it may define environments the
  public file doesn't have at all. A missing private file is normal, not
  an error.

### Changed

- **Breaking:** variable declarations — both in a file's prelude and local
  to a request block — now require an `@` prefix: `@key = value` instead
  of `key = value`, matching JetBrains HTTP Client / httpyac / kulala.
  Update any existing `.http` files; the old bare syntax is no longer
  recognized as a variable declaration.
- **Breaking:** the environment file is now named `http-client.env.json`
  (was `httpfly.env.json`), and its shape has changed: environment names
  are top-level keys directly instead of nested under an `"environments"`
  key, and the shared-defaults bucket is now `"$shared"` (was `"shared"`).
  See [Environments](doc/environments.md) for the new format and the
  private-overlay file above.
- Because the `###` block separator now matches on prefix rather than
  requiring an exact `"###"` line (needed for the separator-line-name
  feature above), a line starting with `###` anywhere in a file —
  including inside a request body — now starts a new block. This matches
  JetBrains HTTP Client's own behavior; there's no way to escape a literal
  `###` inside a body.

## [0.2.0] - 2026-08-24

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
