# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`httpfly.nvim` is a Neovim plugin for sending requests from JetBrains-style
`.http` files (`###` request separators, `{{variables}}`,
`< {% ... %}`/`> {% ... %}` Lua pre/post-request scripts,
`http-client.env.json` environments). It has two halves in one repo:

- **Lua plugin** (repo root: `lua/`, `plugin/`, `ftdetect/`, `ftplugin/` —
  must stay at the root for plugin managers to load it). Discovers/selects
  the environment, shells out to the backend with `httpfly run -json`, and
  renders the JSON into a colored, box-drawn result pane, with a floating
  preview for values too long for a table cell.
- **Go backend** (`backend/`, its own Go module; module path
  `github.com/cristianradulescu/httpfly.nvim/backend`). Does all the heavy lifting:
  parsing, variable substitution, script execution, environment merging,
  sending requests, persisting `client.global` variables. See
  `backend/CLAUDE.md` for its internals — read that before changing Go
  code.

User docs live in `docs/` (`.http` format, environments, scripting, and
`docs/backend.md` for the CLI and `-json` contract), with runnable
examples in `docs/examples/`. `README.md` is the front page and links
into them. Keep user-facing detail in `docs/`, not here.

## Build, check, version

- Root `Makefile` covers both halves. `make build` compiles
  `backend/cmd/httpfly` into `bin/httpfly` (gitignored), injecting
  `git describe --tags` as the version. `make check` runs the gofmt check,
  `go vet`, `go test` and `stylua --check` — run it before considering
  work done.
- One version for the whole repo: the `vX.Y.Z` git tags.
- Any user-visible change (new command, changed default, changed
  resolution behavior, etc.) must get an entry under `## [Unreleased]` in
  `CHANGELOG.md` (Keep a Changelog) as part of the same change — don't
  leave it for a separate pass.
- The Lua side has no test suite and no dependencies; it's verified by
  exercising modules through `nvim --headless` (see "Manual verification"
  below). The backend has a normal `go test` suite.

## Running the backend

Users build it with `build = "make build"` in their lazy.nvim spec.
`lua/httpfly/backend.lua`'s `get_path()` resolves the binary as
`vim.api.nvim_get_runtime_file("bin/httpfly", false)[1]` made absolute —
i.e. the plugin's own directory, wherever it's installed. There's
deliberately no option to point at a different binary: the Lua side and
the backend must come from the same commit. To test a backend change,
load the plugin from the checkout (lazy `dir = ...`) and `make build`. It must be
absolute because httpfly runs with a different `cwd` (below). `resolve()`
additionally checks it's executable; `runner.lua`'s `ensure_binary()` and
`lua/httpfly/health.lua` (`:checkhealth httpfly`) use it and point the
user at `make build` when it's missing. Plugin and backend always ship
together, so there's no version-compatibility checking between them —
change the `-json` output (`backend/internal/cli/json.go`) and the Lua
renderer (`lua/httpfly/format/`) together, in the same commit.

The backend resolves `http-client.env.json`, the optional sibling
`http-client.private.env.json`, `.httpfly/state.json`, and relative paths
(`< file` bodies, `cmd.exec`) against its process's **current working
directory only** — it does no upward search. The plugin does the upward
search: `env.lua`'s `find_env_dir()` walks up from the `.http` file's
directory for `http-client.env.json` (`vim.fs.find(..., { upward = true })`),
and `env.resolve_cwd()` returns that directory — or the `.http` file's own
directory if none is found — as the `cwd` httpfly always runs with. So one
env file at a project root serves `.http` files nested any depth below it,
a closer env file shadows it for its subtree, and `.httpfly/` (state,
history, downloads) is shared by everything under the env file's
directory. The private file is always expected next to the public one,
never searched for independently.

## Architecture

Request flow, end to end:

1. `plugin/httpfly.lua` registers `:HttpEnv`, `:HttpEnvVars`, `:HttpSend`,
   `:HttpSendAll`, `:HttpSessionClear` on load (guarded by
   `vim.g.loaded_httpfly`).
2. `lua/httpfly/env.lua` resolves which environment (a top-level key of
   `http-client.env.json` or of the private overlay) applies to the
   current buffer. `env_file_for_buf()` does the upward search above and
   keeps the selected environment name in a module-local table **keyed by
   that env file's path**, so switching projects doesn't bleed state.
   `private_env_file_for_buf()` reuses the same directory.
   `:HttpEnv` with no argument opens a `vim.ui.select` picker over
   `read_env_names()` — every top-level key across *both* files (so an
   environment defined only in the private file is selectable), except
   `"$shared"`, which the backend also rejects as an environment.
   `:HttpEnv <name>` sets it directly, without validating it exists.
   - `env.status(bufnr)` returns the winbar text; `ftplugin/http.lua` wires
     it up as a **live** winbar expression
     (`%{%v:lua.require('httpfly.env').status()%}`), so it stays correct
     after `:HttpEnv` without a manual redraw.
   - `env.vars(bufnr)` / `env.show_vars(bufnr)` (`:HttpEnvVars`) replicate
     the backend's merge order (`internal/env.Load`) so the preview matches
     a real send, lowest to highest: public `"$shared"` → public `<env>` →
     private `"$shared"` → private `<env>` → persisted `.httpfly/state.json`
     (`session.load()`). The actual send just passes `-env <name>` and lets
     the backend merge.
3. `lua/httpfly/runner.lua` builds the command
   (`httpfly run -json [-env E] [-timeout D] [-download F] [-name X] <file>`)
   and runs it with `vim.system`, `cwd` = `env.resolve_cwd()`.
   - **Sending the request under the cursor**: the backend selects a single
     request only by name (`-name X`, matching the request's mandatory
     `@name`), so `M.send_current()` finds the enclosing request's name
     itself. `parse_blocks()` mirrors the backend parser: split on lines
     starting with `###` (no leading whitespace — the backend uses
     `strings.HasPrefix`); the segment before the first one is the prelude
     and never has a name. A block's name comes from trailing text on its
     `###` line (`### GetUsers`) or a later `# @name X` line, which wins
     (the backend requires them to agree; the plugin just needs *a* name).
     `parse_blocks()` also records each block's request URL and any
     `# @download` annotation. `find_enclosing_block()` picks the block
     containing the cursor line, excluding the `###` line itself. If no
     name is found (prelude, separator line, unnamed block) it
     `vim.notify`s a warning instead of sending.
   - **`# @download`** is a plugin-level annotation on top of the
     backend's `-download F` flag (which applies to exactly one selected
     request and needs the path upfront, so the filename can't come from
     the response's `Content-Disposition`). The backend just reports it as
     unknown metadata. `resolve_download_path()` turns it into a path under
     `<cwd>/.httpfly/downloads/` — the annotation's value if given, else
     `guess_filename()` (the URL's last query-stripped path segment,
     falling back to `"download"`) — creating the directory, since the
     backend doesn't (same as `curl -o`).
   - **`M.send_all()`**: if no block has `@download`, one invocation for
     the whole file (`build_cmd(file)`, the common, fast path). Otherwise
     `run_many()` runs one `httpfly run -name X [-download F] <file>` per
     named block, **sequentially** in file order. That's equivalent to one
     process because the backend writes `client.global:set(...)` through
     to `.httpfly/state.json` immediately, so each invocation sees the
     previous ones' state.
   - `run_many()` concatenates the per-invocation JSON arrays and renders
     them once via `format.render_decoded()` (the decoded-input sibling of
     `format.render()`), so the result buffer/history entry is one unified
     view, with the Command block listing every invocation, one per line.
   - **Persistence** is entirely the backend's (`internal/state`: one file,
     a flat `{key: value}` bucket per environment, `""` for none).
     `lua/httpfly/session.lua` only reads it back for `:HttpEnvVars`
     (`session.load(dir, env_name)`) and deletes the whole file for
     `:HttpSessionClear` (`session.clear(dir)`).
   - stderr: `append_stderr()` shows non-empty stderr as its own section
     even on success (script `print(...)` output goes there; stdout is
     pure JSON). If stdout doesn't decode, `raw_fallback_lines()` shows
     the raw stdout/stderr instead (e.g. a parse error, which the backend
     reports on stderr with no JSON).
4. `lua/httpfly/format.lua` decodes the JSON (`format/shared.lua`'s
   `extract_json`, tolerant of stray text before the `[`), checks it's a
   table, and hands it to the renderer, `format/unicode.lua`
   (`render(decoded, cmd_str)` → `(lines, truncations, highlights)`). The
   result buffer is `filetype = "text"`; history files are `.txt`.
   - **Per-request JSON shape** (see `docs/backend.md#-json`): each array
     element is `{name, request:{method,url,proto,headers,body},
     response:{status_code,headers,body,download_path?,tls?}, final_url?,
     script_error?, error?, duration_ms}`. A request that failed to send
     has `error` instead of `response`, rendered as an "▸ Error" section.
     `script_error` (a failed post-request script) coexists with
     `response` and is rendered as a flagged line after the body. With
     `download_path` set, `response.body` is empty and a "Downloaded to"
     line replaces the response body block.
   - `format/shared.lua`: `truncate()`, `status_category()`/
     `status_badge()`, `is_binary()`, `body_lang()`.
   - **Binary body guard** (`shared.is_binary`): body rendering checks
     `body:find("\0", 1, true)` first and substitutes a placeholder —
     otherwise `history.save()` would crash (`E5108`) when a NUL-containing
     line reaches `vim.fn.writefile()` (the Lua↔VimL bridge turns it into a
     `Blob`, which `writefile()` rejects). Valid text never contains a raw
     NUL, so it's an exact signal.
   - **JSON body highlighting**: `lua/httpfly/json.lua`'s
     `pretty(str, tokens)` (see the file for the token-recording
     mechanism).
5. `runner.lua` writes the rendered lines into a reused scratch buffer
   (`httpfly://result`, vertical split), registers the truncation map with
   `lua/httpfly/preview.lua`, and saves the same output to
   `<cwd>/.httpfly/history/<YYYYMMDD-HHMMSS-microseconds>.txt` via
   `lua/httpfly/history.lua`. History is only written when the JSON parsed;
   the raw fallback isn't saved.
6. `lua/httpfly/preview.lua` implements the `K` keymap (buffer-local,
   `preview_keymap`, bound once when the result buffer is created): a
   floating window with the untruncated value under the cursor, closing on
   cursor move / buffer leave / insert mode.

Filetype/keymap wiring: `ftdetect/http.lua` registers `*.http` as filetype
`http`; `ftplugin/http.lua` sets the winbar (unconditionally) and the
buffer-local keymaps (`<leader>hs`, `<leader>ha`, `<leader>he`, `<leader>hv`,
`<leader>hc`) for send-current, send-all, env-picker, env-vars, and
session-clear, gated by `config.options.keymaps`. There is no dedicated
download keymap/command — `# @download` is picked up automatically by
`:HttpSend`/`:HttpSendAll`.

## Manual verification

`make check` covers the backend and Lua formatting; there's no automated
test runner for the Lua side. To check changes, exercise the relevant
module directly through headless Neovim, e.g.:

```bash
nvim --headless -u NONE \
  -c "set rtp+=." \
  -c "lua local f=io.open('/path/to/sample_httpfly_output.json'); local c=f:read('*a'); f:close(); print(table.concat((require('httpfly.format').render(c)), '\n'))" \
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

To see the actual JSON shape (useful when extending `format.lua`), run
`bin/httpfly run -json <file>` directly (e.g. in `docs/examples/` with
`make httpbin-up`).

## Style

Lua: `.stylua.toml` is checked in; `make format` checks and `make
format-fix` applies it. Files consistently use 2-space indentation. Go:
`gofmt` (`make fmt`), plus `golangci-lint` via `make lint` (config in
`backend/.golangci.yml`).
