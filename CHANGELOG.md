# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
