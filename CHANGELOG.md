# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- httpfly's stderr is now shown in the result buffer as its own
  `stderr` section whenever it's non-empty, even on a successful run —
  since httpfly's post-v0.3.1 change a script's `print(...)` writes there (never to
  stdout, which stays pure JSON), so this is where script debugging
  output lands. Previously stderr was only shown on a non-zero exit.

### Changed

- A post-request script error is now labelled `script error:` instead of
  `post-request script error:` in both renderers — httpfly's own
  `script_error` text already starts with `post-request script:`, so the
  old label read as a doubled prefix.
- Requires an httpfly newer than v0.3.1 for the stderr behavior above; on
  older versions script `print` output still lands in stdout ahead of
  the JSON (tolerated by `extract_json` as long as it contains no `[`).
  That same httpfly change also rejects a file that reuses one `@name` on two
  blocks; `:HttpSend` on such a file shows httpfly's own error in the
  result buffer rather than silently sending the first block.

## [0.3.0] - 2026-10-15

### Added

- `:HttpSend`/`:HttpSendAll` now check that the configured `cmd` (default
  `httpfly`) is actually executable before shelling out to it, and show a
  `vim.notify` error pointing at httpfly's install docs (or `setup({ cmd =
  ... })`) instead of silently failing with a raw "command not found" in the
  result buffer.
- A request's `@name` can now be resolved from trailing text on its own
  `###` separator line (`### GetUsers`), matching httpfly v0.3.0's new
  shorthand for `# @name GetUsers` — `:HttpSend` on such a block no longer
  reports "no request (@name) found under cursor". An explicit `# @name`
  line later in the same block still takes precedence.
- New `private_env_file` option (default `http-client.private.env.json`),
  matching httpfly v0.3.0's optional environment-file overlay for values
  you don't want committed (credentials, personal tokens, a local-only
  environment). `:HttpEnv`'s picker lists environments defined in either
  file, and `:HttpEnvVars` shows the full precedence chain (public
  `$shared` < public `<env>` < private `$shared` < private `<env>` <
  persisted session state).

### Changed

- **Breaking:** `env_file` now defaults to `http-client.env.json` (was
  `httpfly.env.json`), matching httpfly v0.3.0's env-file rename. The
  file's shape also changed to match, and this plugin's own parsing now
  assumes the new shape unconditionally (regardless of `env_file`'s
  value): environment names are top-level keys directly instead of
  nested under an `"environments"` key, and the shared-defaults bucket is
  now `"$shared"` (was `"shared"`). Rename and reshape any existing env
  file to match; this plugin now requires httpfly v0.3.0+.

### Removed

- `doc/examples/` (the runnable `.http` example files and their
  `httpfly.env.json`) and the `Makefile`'s `httpbin-*` targets that
  supported them — the examples now live in httpfly's own repo. `README.md`
  points there instead.

### Changed

- `httpfly.env.json` no longer has to sit directly alongside the `.http`
  files that use it. The plugin now searches upward from a `.http` file's
  own directory to find it (`vim.fs.find(..., upward = true)`) and launches
  httpfly with that directory as `cwd`, so one env file at a project's root
  can serve `.http` files nested arbitrarily far below it (e.g.
  `v1/request.http`, `v2/request.http`). `.httpfly/state.json` and
  `.httpfly/history/` follow the same resolved directory, so persisted
  session state and history are shared by every `.http` file under it.
  Falls back to the `.http` file's own directory when no env file is found
  anywhere upward, matching the previous behavior.

## [0.2.0] - 2026-08-24

### Added

- `# @download` / `# @download some-name.ext` annotation to save a
  request's response body to disk, byte-perfect, restoring the old
  httpyac-backed plugin's design on top of httpfly's new `-download` flag.
  Detected automatically by both `:HttpSend` and `:HttpSendAll` — no
  separate command. Saved under `.httpfly/downloads/`, filename from the
  annotation's value or guessed from the URL when bare (httpfly needs the
  destination path before sending, so — unlike a browser — there's no way
  to name the file from the response's `Content-Disposition`).

### Changed

- Switched the backend from [httpyac](https://httpyac.github.io/) to
  [httpfly](https://github.com/cristianradulescu/httpfly), a purpose-built
  CLI for this plugin. `cmd` now defaults to `httpfly`; `env_file` now
  defaults to `httpfly.env.json`.
- Environment files: a single `httpfly.env.json` (`{"shared": {...},
  "environments": {...}}`) replaces `http-client.env.json` plus its
  `.private.` counterpart. httpfly resolves it (and `.httpfly/state.json`)
  relative to its own process cwd only, with no upward directory search —
  the file must sit directly alongside the `.http` files that use it.
- Pre-/post-request scripts (`< {% ... %}` / `> {% ... %}`) are now Lua,
  not JavaScript — the only language httpfly currently supports.
- `:HttpSend` (send the request under the cursor) now resolves the
  enclosing request's `# @name` and calls `httpfly run -name X`, replacing
  the previous line-based `httpyac send --line N`.
- Cross-invocation variable persistence (`client.global:set(...)` surviving
  across separate `:HttpSend` calls) is now native to httpfly
  (`.httpfly/state.json`) — no bundled plugin hook required.

### Removed

- `@download` support for saving binary responses to disk — httpfly has no
  plugin/hook mechanism to build this on, unlike httpyac. May return if/when
  httpfly grows native download support.
- The "Test Results" rendered section — httpfly has no `client.test(...)`-
  style assertion API, so there's nothing to render there. Post-request
  script errors are still surfaced, now as their own flagged line.

## [0.1.0] - 2026-08-23

Initial release.

### Added

- `:HttpSend` / `:HttpSendAll` to send the request under the cursor, or every
  request in the file, via `httpyac send --json --no-color`.
- `:HttpEnv` to discover and pick the httpyac environment
  (`http-client.env.json`, including its `.private.` counterpart) for the
  current buffer, shown live in the winbar.
- `:HttpEnvVars` to preview the merged variables (shared/env/session layers)
  that a real send would see, highlighting session-added and
  session-overridden values.
- `:HttpSessionClear` to clear cross-invocation session state.
- Markdown and unicode (box-drawing) output styles, with JSON body
  pretty-printing and syntax highlighting.
- Floating-window preview (`K`) for values truncated in header tables, with a
  second `K` to focus into the popup for copying text.
- Per-request history saved under `.httpfly/history/`.
- Cross-invocation variable persistence (e.g. MFA tokens set by one request's
  script, reused by a later request sent separately) via an httpyac plugin
  hook, with zero changes required to existing `.http` files.
- `@download` support for saving binary responses to disk, with
  browser-style filename resolution and path-traversal protection.
- Autosave of the buffer before sending, so httpyac never sends a stale
  on-disk version of an unsaved edit.

[0.1.0]: https://github.com/cristianradulescu/httpfly.nvim/releases/tag/v0.1.0
