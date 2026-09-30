# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`httpfly.nvim` is a Neovim plugin that provides a thin UI layer over the
httpfly CLI — its own Go backend, bundled in this repo under `backend/` —
for executing
JetBrains-style `.http` files (`###` request separators, `{{variables}}`,
`< {% ... %}`/`> {% ... %}` Lua pre/post-request scripts,
`http-client.env.json` environments). httpfly does all the heavy lifting — sending requests,
variable substitution, script execution, environment merging, persisting
`client.global` variables across separate invocations. This plugin's job
is: discover/select the environment, shell out to `httpfly run -json`, and
turn its JSON output into a colored, box-drawn result pane in a split,
with a floating-window preview for values too long to fit in a table cell.

## Repo layout

- Lua plugin at the repo root (`lua/`, `plugin/`, `ftdetect/`,
  `ftplugin/`) — must stay at the root for plugin managers to load it.
- `backend/` — the httpfly Go module (`backend/go.mod`, module path
  `github.com/cristianradulescu/httpfly`, kept from when it was a
  standalone repo; nothing `go get`s it). It has its own
  `backend/CLAUDE.md` covering its internals — read that before changing
  Go code. Its user docs (`.http` format, environments, scripting, `-json`
  contract) and runnable examples live in `backend/doc/`.
- Root `Makefile` covers both halves: `make build` compiles
  `backend/cmd/httpfly` into `bin/httpfly` (gitignored), injecting the
  plugin's own `git describe --tags` as the version; `make check` runs
  the gofmt check, `go vet`, `go test` and `stylua --check`. Run `make
  check` before considering work done.
- One version for the whole repo: the plugin's `vX.Y.Z` tags. The
  backend's former standalone releases are frozen in
  `backend/CHANGELOG.md`; new changes on either side go in the root
  `CHANGELOG.md`.

The Lua side has no test suite and no dependencies; it's verified by
exercising modules directly through `nvim --headless` (see "Manual
verification" below). The backend has a normal `go test` suite.

Any user-visible change (new command, changed default, changed resolution
behavior, etc.) must get an entry under `## [Unreleased]` in `CHANGELOG.md`
(Keep a Changelog format, already in use there) as part of the same change
— don't leave it for a separate pass.

## Runtime dependency

The plugin runs the bundled `bin/httpfly`, built by `make build` (users
put `build = "make build"` in their lazy.nvim spec). `lua/httpfly/backend.lua`'s
`get_path()` resolves it: `config.options.cmd` if set (an override, e.g. a
dev build), else `vim.api.nvim_get_runtime_file("bin/httpfly", false)[1]`
— i.e. the plugin's own directory, wherever it's installed. `resolve()`
additionally checks it's executable; `runner.lua`'s `ensure_binary()` and
`lua/httpfly/health.lua` (`:checkhealth httpfly`) both use it and point
the user at `make build` when it's missing. Because the plugin and backend
ship together, there's no version-compatibility checking between them —
change the `-json` output (`backend/internal/cli/json.go`) and the Lua
renderers (`lua/httpfly/format/`) together, in the same commit.

httpfly resolves `http-client.env.json` (plus an optional sibling
`http-client.private.env.json` overlay, for values that shouldn't be
committed), and `.httpfly/state.json`, relative to its own process's
**current working directory only** — httpfly itself does no upward
directory search the way some other HTTP-file tools do. This plugin
compensates by doing the upward search itself: `env.lua`'s
`find_env_dir()` walks up from the `.http` file's own directory looking for
`http-client.env.json` (via `vim.fs.find(..., { upward = true })`), and
`env.resolve_cwd()` returns that directory — falling back to the `.http`
file's own directory if no env file is found anywhere upward — as the `cwd`
this plugin always launches httpfly with. This lets one
`http-client.env.json` at a project's root serve `.http` files nested
arbitrarily far below it (e.g. `v1/request.http`, `v2/request.http`); a
subtree that needs a genuinely different environment file still just keeps
its own copy closer to those `.http` files, which shadows the root one for
anything under it. Because `.httpfly/state.json` is also resolved relative
to that same `cwd`, persisted `client.global` state is shared by every
`.http` file under the env file's directory, not scoped per subdirectory.
The private overlay, if present, is always expected alongside the public
file in that same resolved directory — this plugin never searches for it
independently.

## Architecture

Request flow, end to end:

1. `plugin/httpfly.lua` registers `:HttpEnv`, `:HttpEnvVars`, `:HttpSend`,
   `:HttpSendAll`, `:HttpSessionClear` on load (guarded by
   `vim.g.loaded_httpfly`).
2. `lua/httpfly/env.lua` resolves which httpfly environment (a top-level
   key of `http-client.env.json`, or of its optional
   `http-client.private.env.json` overlay) applies to the current buffer.
   Because httpfly itself does no upward search (see above),
   `env_file_for_buf()` does one itself — `vim.fs.find(config.options.env_file,
   { path = dir, upward = true })` from the buffer's own directory up to the
   filesystem root — and keeps the selected environment name in a
   module-local table **keyed by that env file's path**, not globally, so
   switching directories/projects doesn't bleed state.
   `private_env_file_for_buf()` reuses the same resolved directory for
   `config.options.private_env_file`, without searching independently —
   the private file, when present, always sits alongside the public one.
   `:HttpEnv` with no argument opens a `vim.ui.select` picker over
   `read_env_names()`'s result — every top-level key across *both* files,
   unioned so an environment defined only in the private file (e.g. a
   personal `local`) is selectable too, except `"$shared"`, which isn't
   itself selectable, same as httpfly's own CLI rejecting `-env $shared`.
   `:HttpEnv <name>` sets it directly, without going through the
   picker/`read_env_names()` at all (and without validating the name
   exists anywhere).
   - `env.status(bufnr)` returns the winbar text; `ftplugin/http.lua` wires
     it up as a **live** winbar expression
     (`%{%v:lua.require('httpfly.env').status()%}`), not a value set once
     at buffer-load time, so it stays correct after `:HttpEnv` changes the
     selection without needing to manually redraw anything.
   - `env.vars(bufnr)` / `env.show_vars(bufnr)` (bound to `:HttpEnvVars`)
     replicate httpfly's own merge order (`internal/env.Load`) so the
     values shown match what a real send actually uses, lowest to highest:
     public `"$shared"` → public `<env>` → private `"$shared"` → private
     `<env>` → persisted `.httpfly/state.json` (`session.load()`, see
     below), each layer overwriting the last. `runner.lua`'s `build_cmd()`
     just passes `-env <name>` once and lets httpfly itself do this same
     merge server-side for the actual send — `env.vars()` only needs to
     reproduce it locally so `:HttpEnvVars` can preview it without
     sending anything.
3. `lua/httpfly/runner.lua` builds the httpfly command
   (`httpfly run -json [-env E] [-timeout D] [-download F] [-name X] <file>`) and runs it with
   `vim.system`, with `cwd` set to `env.resolve_cwd()` — the directory this
   plugin's own upward search (see above) found `http-client.env.json` in,
   or the `.http` file's own directory if none was found — since that's the
   only directory httpfly itself will look in for
   `http-client.env.json`/`.httpfly/state.json`. Unlike the previous
   httpyac-backed version, no extra environment variables need to be
   injected into the child process at all: httpfly persists
   `client.global` state natively (see below), so there's no bundled
   plugin/hook file to point at via an env var, and no `cd ...&&` prefix
   is needed in the rendered command string either — `vim.system`'s own
   `cwd` option is sufficient.
   - **`-name` instead of `--line N`**: httpfly has no line-based "send
     the request under the cursor" flag — `-name X` (matching the request's
     mandatory `@name`) is the only way to restrict a run to one request.
     `M.send_current()` therefore has to find the enclosing request's name
     itself: `find_enclosing_block()`/`parse_blocks()` mirror httpfly's own
     parser, which splits the file on lines starting with `###` — no
     leading whitespace, matching httpfly's `strings.HasPrefix` (the
     segment before the first one is the prelude and never carries a
     `@name`, since `@name` is request-only and mandatory on every real
     request). Since httpfly v0.3.0, a block's `@name` can come from
     trailing text on its own `###` separator line (`### GetUsers`,
     equivalent to a bare `###` followed by `# @name GetUsers`) as well as
     from an explicit `# @name X` line later in the block — `parse_blocks()`
     checks the separator line first and lets an explicit `# @name` later
     in the same block override it (httpfly itself requires the two to
     agree when both are present; this plugin doesn't validate that, it
     just needs *a* name to pass to `-name`). `parse_blocks()` walks every
     block once, extracting its resolved name, its request line's URL (for
     a download filename guess — see below), and an `# @download`
     annotation if present; `find_enclosing_block()` picks out whichever
     block contains the cursor line — note this excludes the block's own
     `###` separator line, so a cursor sitting exactly on `### GetUsers`
     itself (rather than inside the block) still won't resolve, same as
     before this block could carry a name at all. `vim.notify`s a warning
     rather than sending anything if no name is found (cursor sitting in
     the prelude, on a separator line itself, or in an `.http` file that
     hasn't declared a name yet).
   - **`# @download`**: httpfly's own equivalent is a plain `-download F`
     flag on `run` (added after this plugin's initial httpyac→httpfly
     switch), not a per-request annotation — it only ever applies to a
     single selected request, and needs the destination path upfront,
     before the request is even sent (so, unlike a browser or the old
     httpyac-backed version of this plugin, there's no way to name the file
     from the *response*'s `Content-Disposition` — only the URL is
     available at build-command time). This plugin restores a
     `# @download` / `# @download some-name.ext` annotation on top of that
     flag, matching the old httpyac-backed plugin's design: `parse_blocks()`
     recognizes it as its own metadata key (unknown to httpfly itself, which
     just reports it as a harmless "unknown metadata" warning), and
     `resolve_download_path()` turns it into a full path under
     `.httpfly/downloads/` next to the `.http` file — `guess_filename()`
     (the URL's last `/`-delimited, query-stripped path segment, falling
     back to `"download"`) when the annotation is bare, the annotation's
     value verbatim otherwise — creating that directory if needed (httpfly
     itself does not create `-download`'s target directory, same as
     `curl -o`). `M.send_current()` resolves this for the single block it
     found and passes it straight into `build_cmd()`'s optional
     `download_path` argument.
   - **`M.send_all()` and mixed `@download` files**: httpfly's `-download`
     requires selecting exactly one request, so a file where only *some*
     requests are marked `@download` can't be sent as one
     `httpfly run <file>` call the way `:HttpSendAll` normally does.
     `send_all()` checks `parse_blocks()` for any `@download` first line: if
     none, it takes the fast path unchanged (`build_cmd(file)`, no
     `-name`, one process for the whole file — this is the common case and
     is not slowed down by any of this). If at least one block is marked,
     it instead calls `run_many()` with one `build_cmd(file, name,
     download_path)` per named block, each its own `httpfly run -name X
     [-download F] <file>` invocation, sent **sequentially** in file order.
     This is still functionally equivalent to one process for the whole
     file: httpfly writes `client.global:set(...)` through to
     `.httpfly/state.json` immediately (see below), the same mechanism that
     already makes a login→token chain work across separate `:HttpSend`
     calls, so N sequential invocations see each other's persisted state
     exactly like one process would — just slower (N process spawns
     instead of one).
   - `run_many()` stitches the resulting per-invocation JSON arrays into one
     combined array (`format.render_decoded()`, `format.lua`'s decoded-input
     sibling to `format.render()`, added specifically so this stitching
     doesn't need to round-trip back through JSON text) and renders that
     once, so the result buffer/history entry reads as a single unified
     view — same as `run()`'s single-invocation path — with a `**Command**`
     block listing every invocation's command line, one per line (both
     renderers already split `cmd_str` on `"\n"` before inserting it, which
     `run()`'s always-single-line `cmd_str` also satisfies trivially).
   - **Cross-invocation variable persistence** is native to httpfly —
     `client.global:set(...)` in a Lua script writes straight through to
     `.httpfly/state.json` (`internal/state` on httpfly's side), keyed by
     directory and environment name, immediately on every `:set`, not just
     at the end of a run. This plugin doesn't need to do anything to make
     that work; `lua/httpfly/session.lua` only exists so `:HttpEnvVars` can
     *read* that same file back (`session.load(dir, env_name)` — a flat
     `{key: value}` bucket per environment, `""` for no `-env`) to show
     persisted overrides alongside env-file variables, and so
     `:HttpSessionClear` can delete it (`session.clear(dir)` — deletes the
     whole file, same coarse granularity as before; httpfly's per-
     environment buckets all live in that one file).
4. `lua/httpfly/format.lua` is a thin dispatcher: it decodes the JSON
   payload (`format/shared.lua`'s `extract_json`, defensive against any
   stray non-JSON text before the `[` — relevant if `cmd` is ever invoked
   through something that prints notices to stdout first; httpfly itself
   keeps `-json` stdout clean — since its post-v0.3.1 change a script's `print(...)` and
   flag errors go to stderr, which `runner.lua`'s `append_stderr()` shows
   as its own section even on a successful run). httpfly's
   `-json` output is a **top-level array** (not `{summary, requests}` the
   way the previous httpyac backend's was) — `format.lua` just checks the
   decoded value is a table before handing it to the one renderer,
   `format/unicode.lua` (`render(decoded, cmd_str)` → `(lines,
   truncations, highlights)`). The result buffer's `filetype` (`"text"`)
   and history extension (`.txt`) are set directly in
   `runner.lua`/`history.lua`.
   - **Per-request JSON shape**: each array element is
     `{name, request:{method,url,proto,headers,body},
     response:{status_code,headers,body,download_path?,tls?}, final_url?,
     script_error?, error?, duration_ms}`. `name` is always present
     (httpfly makes `@name` mandatory on every request, so there's no
     `fileName` fallback to fall back to the way the old httpyac backend
     needed). A request that failed to *send* (DNS failure, connection
     refused, ...) has `error` instead of `response` — both renderers
     explicitly render an "Error"/"▸ Error" section for this now, since
     silently rendering nothing when there's no response object would
     otherwise hide every transport failure. `script_error` (a
     post-request script that errored) can coexist with a present
     `response` — rendered as its own flagged line right after the
     response body, matching httpfly's own docs ("coexists with
     `response`, unlike `error`"). When the request was sent with
     `-download` (i.e. it carried `# @download`), `response.download_path`
     is set and `response.body` is empty — both
     renderers check `download_path` first and render a "Downloaded to"
     line with that path in place of the normal body block (the request
     body, if any, still renders normally — only the response is affected
     by `-download`).
   - **No test-assertion output**: httpfly has no `client.test(...)`-style
     API and so no `testResults` field in its JSON at all — the "Test
     Results" section that existed under the httpyac backend has been
     removed from both renderers, not repurposed. If httpfly ever grows
     assertions, this is where that rendering would come back.
   - `format/shared.lua` holds what's genuinely style-independent:
     `truncate()`, `status_category()`/`status_badge()`, `is_binary()`,
     `body_lang()` — unchanged from the httpyac backend, since they only
     depend on generic HTTP concepts (status codes, headers, a body
     string), not anything httpyac- or httpfly-specific.
   - **Binary body guard** (`shared.is_binary`): both renderers' body
     rendering checks `body:find("\0", 1, true)` before doing anything else
     with a response body, substituting a placeholder if found — otherwise
     `history.save()` would crash (`E5108`) the moment a NUL-byte-containing
     body reached `vim.fn.writefile()`'s line-list argument (Neovim's own
     Lua↔VimL bridge silently promotes a NUL-containing string element to a
     `Blob` there, which `writefile()` rejects outright). A NUL byte is a
     sufficient, exact signal that content isn't real text — valid
     JSON/XML/HTML/plain text never contains one raw.
   - **JSON body syntax highlighting**: via `lua/httpfly/json.lua`'s
     `pretty(str, tokens)` — unchanged from before, still style-independent
     (see the file itself for the token-recording mechanism).
5. `lua/httpfly/runner.lua` writes the rendered lines into a reused scratch
   buffer (`httpfly://result`, opened in a vertical split), registers the
   truncation map with `lua/httpfly/preview.lua`, and saves the same
   rendered output to `<dir>/.httpfly/history/<YYYYMMDD-HHMMSS-microseconds>
   .txt` via
   `lua/httpfly/history.lua`, where `<dir>` is the same directory
   `env.resolve_cwd()` computed for this send, matching where httpfly's own
   `.httpfly/state.json` lives, so a project only needs one `.gitignore`
   entry to cover both. History is
   only written when httpfly's JSON parsed successfully — the raw-fallback
   path (httpfly crashed before emitting JSON) is not saved since there's
   nothing useful to keep.
6. `lua/httpfly/preview.lua` implements the `K` keymap (buffer-local,
   configurable via `preview_keymap`, bound once when the result buffer is
   created) that opens a floating window with the untruncated value under
   the cursor, closing on cursor move / buffer leave / insert mode.

Filetype/keymap wiring: `ftdetect/http.lua` registers `*.http` as filetype
`http`; `ftplugin/http.lua` sets the winbar (unconditionally) and the
buffer-local keymaps (`<leader>hs`, `<leader>ha`, `<leader>he`, `<leader>hv`,
`<leader>hc`) for send-current, send-all, env-picker, env-vars, and
session-clear, gated by `config.options.keymaps`. There is no dedicated
download keymap/command — `# @download` is picked up automatically by
`:HttpSend`/`:HttpSendAll`, see above.

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

To see httpfly's actual JSON shape (useful when extending `format.lua`),
run `bin/httpfly run -json <file>` directly (e.g. against
`backend/doc/examples/*.http` with `make httpbin-up`) and inspect the
top-level array's `request`/`response`/`error`/`script_error` fields (see
`backend/doc/usage.md#-json`).

## Style

Lua: `.stylua.toml` is checked in; `make format` checks and `make
format-fix` applies it. Files consistently use 2-space indentation. Go:
`gofmt` (`make fmt`), plus `golangci-lint` via `make lint` (config in
`backend/.golangci.yml`).
