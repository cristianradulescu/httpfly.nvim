# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`httpfly.nvim` is a Neovim plugin that provides a thin UI layer over the
[httpyac](https://httpyac.github.io/) CLI for executing JetBrains-style
`.http` files (`###` request separators, `{{variables}}`, `> {% ... %}`
pre/post-request scripts, `http-client.env.json` environments). httpyac does
all the heavy lifting — sending requests, variable substitution, script
execution, environment merging. This plugin's job is: discover/select the
environment, shell out to `httpyac send --json --no-color`, and turn its JSON
output into readable markdown in a split, with a floating-window preview for
values too long to fit in a markdown table cell.

There is no build step and no test suite — this is a small, dependency-free
Lua plugin. Verification is done by exercising modules directly through
`nvim --headless` (see "Manual verification" below).

## Runtime dependency

The plugin assumes `httpyac` is installed and on `$PATH` (`npm i -g httpyac`).
The binary name is configurable via `require("httpfly").setup({ cmd = ... })`.

## Architecture

Request flow, end to end:

1. `plugin/httpfly.lua` registers `:HttpEnv`, `:HttpEnvVars`, `:HttpSend`,
   `:HttpSendAll`, `:HttpSessionClear` on load (guarded by
   `vim.g.loaded_httpfly`).
2. `lua/httpfly/env.lua` resolves which httpyac environment (a key in
   `http-client.env.json`) applies to the current buffer. It walks upward
   from the buffer's directory to find the nearest `http-client.env.json`
   (`vim.fs.find(..., { upward = true })`) and keeps the selected environment
   name in a module-local table **keyed by that env file's path**, not
   globally — so switching directories/projects doesn't bleed state.
   `:HttpEnv` with no argument opens a `vim.ui.select` picker over that
   file's top-level keys; `:HttpEnv <name>` sets it directly.
   - `env.status(bufnr)` returns the winbar text; `ftplugin/http.lua` wires
     it up as a **live** winbar expression
     (`%{%v:lua.require('httpfly.env').status()%}`), not a value set once
     at buffer-load time, so it stays correct after `:HttpEnv` changes the
     selection without needing to manually redraw anything.
   - `env.vars(bufnr)` / `env.show_vars(bufnr)` (bound to `:HttpEnvVars`)
     replicate httpyac's own merge order so the values shown match what a
     real send actually uses: shared file's `"$shared"` → shared file's
     selected env → private file's `"$shared"` → private file's selected
     env → session vars (`session.load()`, see below), each layer
     overwriting the last. The private file's path is derived from
     `config.options.env_file` by suffix substitution
     (`http-client.env.json` -> `http-client.private.env.json`), not a
     separate config option. `env.vars()` returns `vars, name, session_keys,
     env_keys` — the latter two are the set of keys present in the session
     and the set already present from the env file(s) alone (before the
     session was applied), so `show_vars()` can tell "session added a new
     var" (`[session]`) apart from "session overrode an existing env var"
     (`[overridden by session]`) and highlight those suffixes
     (`WarningMsg`) via an extmark namespace on the floating window's
     scratch buffer.
3. `lua/httpfly/runner.lua` builds the httpyac command
   (`httpyac send <file> --json --no-color [--line N | --all] [--env E]`)
   and runs it with `vim.system`. It always requests `--json` output — the
   plain-text renderer is never parsed. The `<file>` argument is always the
   buffer's absolute path, but the process `cwd` (`resolve_cwd()`) is the
   **env file's directory**, not the `.http` file's own directory: httpyac
   resolves `http-client(.private).env.json` (and `process.cwd()` inside
   scripts) relative to its process cwd, not relative to the file being
   sent, so a request nested below the env file (e.g. `v2/request.http`
   with the env file at the project root) would silently lose all its
   variables if cwd were the request's own directory. Falls back to the
   file's own directory only when no env file was found for the buffer. The
   exact command (with a `cd` to that same cwd, plus the `HTTPYAC_PLUGIN`
   env var below) is rendered into the output via
   `format.render(stdout, cmd_str)` so it's directly copy-pasteable for
   debugging.
   - **Cross-invocation variable persistence** (`lua/httpfly/session.lua`,
     `httpyac-plugin/session-persist.js`): `client.global.set(...)` in an
     httpyac script only lives for the duration of one `httpyac` process by
     default, so a variable set by request A's script (e.g. an MFA token)
     is invisible to request B if they're sent separately via two
     `:HttpSend` calls — confirmed empirically against a real httpyac
     binary. An earlier approach asked users to have each script
     explicitly persist its own variables via `require("fs")`; that was
     rejected as not scaling to a large existing collection of `.http`
     files, since it required editing every one of them.
     The current approach needs **zero changes to any `.http` file or
     script**, discovered by reading httpyac's own source
     (`registerPlugins`/`configureHooks`, `models/sessionStore.d.ts`):
     httpyac already keeps `client.global` state in an in-memory
     `userSessionStore` singleton for the lifetime of one process — that's
     the actual mechanism `--all` relies on to share state across requests
     in a single run. A project can register a `configureHooks(api)`
     function (via a local `.httpyac.js`, or via the `HTTPYAC_PLUGIN` env
     var pointing at any JS module — httpyac merges both, they don't
     conflict), and `api.sessionStore` is that same singleton. So
     `httpyac-plugin/session-persist.js` (loaded via `HTTPYAC_PLUGIN`, set
     by `runner.lua` on every `vim.system` call, resolved from this
     plugin's own install path via `session.plugin_path()` using
     `debug.getinfo`) just mirrors it to `.httpfly/session.json` next to
     the env file (creating the `.httpfly/` directory if needed — the JS
     side does this itself with `fs.mkdirSync(..., { recursive: true })`
     since it writes into a subdirectory now, not the cwd directly): load
     it into the store on startup, write it back out on
     every `sessionStore.onSessionChanged()`. httpyac's own `{{var}}`
     resolution and `client.global` API do the rest, completely
     unmodified — this was verified against a real httpyac binary across
     two genuinely separate processes before being adopted. Only sessions
     whose `type` ends in `global_cache` are persisted (empirically
     determined — `sessionStore.userSessions` also holds transient
     per-connection sessions with live sockets that aren't
     JSON-serializable and would throw on a circular-structure error if
     included). `:HttpSessionClear` (`session.clear(cwd)`) deletes the
     session file. `session.load(cwd)` (same file-parsing/filtering logic,
     read-only) lets `env.vars()` show these values merged into
     `:HttpEnvVars` output, distinct from the JS plugin above, which
     handles the actual write side.
4. `lua/httpfly/format.lua` turns that JSON payload into markdown: one `##`
   section per request, with method/URL/status line, request/response header
   tables, and body code blocks. Two things it does deliberately, not
   incidentally:
   - **Header table truncation**: values longer than
     `config.options.max_header_value_len` are truncated with `…` (long
     bearer tokens etc. otherwise break table rendering in
     `render-markdown.nvim`, which is why this exists). The full value is
     recorded in a `line number -> full value` map returned alongside the
     rendered lines.
   - **JSON body pretty-printing**: via `lua/httpfly/json.lua`, a
     bracket-scanning re-indenter (not `vim.json.decode` + re-encode) so that
     object key order and string contents are preserved exactly — decoding
     to a Lua table would lose key order since Lua tables are unordered.
5. `lua/httpfly/runner.lua` writes the rendered lines into a reused scratch
   buffer (`httpfly://result`, opened in a vertical split), registers the
   truncation map with `lua/httpfly/preview.lua`, and saves the same
   markdown to `.httpfly/history/<YYYYMMDD-HHMMSS-microseconds>.md` under
   `vim.fn.getcwd()` via `lua/httpfly/history.lua`. History and the session
   file (above) deliberately share the single `.httpfly/` directory so a
   project only needs one `.gitignore` entry to cover both. History is only
   written
   when httpyac's JSON parsed successfully — the raw-fallback path (httpyac
   crashed before emitting JSON) is not saved since there's nothing useful to
   keep.
6. `lua/httpfly/preview.lua` implements the `K` keymap (buffer-local,
   configurable via `preview_keymap`, bound once when the result buffer is
   created) that opens a floating window with the untruncated value under the
   cursor, closing on cursor move / buffer leave / insert mode.

Filetype/keymap wiring: `ftdetect/http.lua` registers `*.http` as filetype
`http`; `ftplugin/http.lua` sets the winbar (unconditionally) and the
buffer-local keymaps (`<leader>hs`, `<leader>ha`, `<leader>he`, `<leader>hv`,
`<leader>hc`) for send-current, send-all, env-picker, env-vars, and
session-clear, gated by `config.options.keymaps`.

## Manual verification

There's no automated test runner. To check changes, exercise the relevant
module directly through headless Neovim, e.g.:

```bash
nvim --headless -u NONE \
  -c "set rtp+=." \
  -c "lua local f=io.open('/path/to/sample_httpyac_output.json'); local c=f:read('*a'); f:close(); print(table.concat((require('httpfly.format').render(c)), '\n'))" \
  -c "qa"
```

`lua/httpfly/json.lua` is pure Lua (no `vim.*` calls) and can be exercised
directly with the system `lua`/`lua5.1` interpreter without Neovim at all —
useful for quickly iterating on the pretty-printer.

To sanity-check filetype detection and keymaps together, `filetype plugin on`
must be set explicitly when using `-u NONE` (bare headless invocations don't
enable it by default the way a real config does):

```bash
nvim --headless -u NONE -c "filetype plugin on" -c "set rtp+=." \
  -c "runtime! plugin/httpfly.lua" -c "edit sample.http" \
  -c "lua print(vim.bo.filetype)" -c "qa"
```

To see httpyac's actual JSON shape (useful when extending `format.lua`),
run `httpyac send <file> --all --json --no-color` directly and inspect
`requests[].response.{statusCode,headers,body,request}` and
`requests[].testResults[]` (present on script assertion failures/errors, with
no `response` key at all on hard failures like DNS errors).

## Style

No `.stylua.toml` is checked in; `stylua` (available on this machine) can be
run with its defaults. Files consistently use 2-space indentation.
