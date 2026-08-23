# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
