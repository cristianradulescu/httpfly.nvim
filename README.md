# httpfly.nvim

Send `.http` requests from Neovim using
[httpfly](https://github.com/cristianradulescu/httpfly) and view the
response in a colored, box-drawn result pane.

![httpfly.nvim showing a request file next to the rendered response](screenshot.png)

> **New here?** Check out httpfly's own
> [`doc/examples/`](https://github.com/cristianradulescu/httpfly/tree/main/doc/examples)
> for runnable `.http` files covering everything below, from basic requests
> to scripting.

## Scope

This plugin does **not** implement request execution, variables, or
scripting itself — [httpfly](https://github.com/cristianradulescu/httpfly)
does that. This plugin just wires it into Neovim:

- discovers/selects the httpfly environment (`http-client.env.json`, plus
  an optional `http-client.private.env.json` overlay) for the current file
- runs `httpfly run` on the request under your cursor (or the whole file)
- formats the JSON result (request/response headers as a table, bodies
  pretty-printed and syntax-highlighted when JSON, status line) in a split
- lets you preview values that got truncated in header tables (e.g. long
  bearer tokens) in a floating window
- saves a copy of every result under `.httpfly/history/`
- saves a response body to disk byte-perfect for any request marked
  `# @download`

If you need JetBrains HTTP Client-style syntax (`###` separators,
`{{variables}}`, `< {% ... %}`/`> {% ... %}` Lua pre/post-request scripts,
`http-client.env.json` environments), that comes from httpfly — this
plugin doesn't reimplement or restrict any of it.

See httpfly's own
[`doc/examples/`](https://github.com/cristianradulescu/httpfly/tree/main/doc/examples)
for runnable `.http` files covering plain GET/POST, pre-/post-request
scripting (including a login → token → authenticated-request chain),
environments, shelling out to a script for auth tokens, forms
(`application/x-www-form-urlencoded` and `multipart/form-data`), saving a
response body for its own sake, and downloading a response to disk via
`# @download`.

## Requirements

- Neovim 0.10+
- [httpfly](https://github.com/cristianradulescu/httpfly) newer than
  v0.3.1 on your `$PATH` (this plugin assumes httpfly's
  `http-client.env.json` env-file format, introduced in v0.3.0, and its
  post-v0.3.1 behavior of keeping `-json` stdout clean by sending script
  `print(...)` output to stderr — an older httpfly won't work correctly
  with it):
  ```sh
  go install github.com/cristianradulescu/httpfly/cmd/httpfly@latest
  ```
  (or build it from source — see httpfly's own `doc/installation.md`)

## Setup

With [lazy.nvim](https://github.com/folke/lazy.nvim):

```lua
{
  "cristianradulescu/httpfly.nvim",
  ft = "http",
  opts = {},
}
```

Calling `require("httpfly").setup({})` (or passing `opts = {}` above) isn't
required — all options have defaults — but it's the way to override them:

```lua
require("httpfly").setup({
  cmd = "httpfly",                             -- httpfly binary/command to run
  env_file = "http-client.env.json",           -- environment file name to look for
  private_env_file = "http-client.private.env.json", -- optional overlay, alongside env_file
  keymaps = true,                              -- set the default <leader>h* keymaps below
  max_header_value_len = 100,                  -- header table cell truncation length
  preview_keymap = "K",                        -- keymap to preview a truncated value
  timeout = nil,                               -- per-request timeout as a Go duration ("2m"); nil = httpfly's 30s default
})
```

## Usage

Open a `.http` file and:

| Command         | Keymap        | Does                                                |
|-----------------|---------------|------------------------------------------------------|
| `:HttpEnv`      | `<leader>he`  | Pick the httpfly environment for this file            |
| `:HttpEnv dev`  | —             | Set the environment directly, without the picker      |
| `:HttpEnvVars`  | `<leader>hv`  | Show the selected environment's merged variables       |
| `:HttpSend`     | `<leader>hs`  | Send the request under the cursor                     |
| `:HttpSendAll`  | `<leader>ha`  | Send every request in the file                        |
| `:HttpSessionClear` | `<leader>hc` | Clear persisted `client.global` state (see below) |

The response opens in a vertical split (`filetype = "text"`): headers as a
table, request/response sections and a body block, drawn with Unicode
box-drawing characters (`┌─┬─┐`, `━━━`) and colored (status, method,
section titles, JSON body syntax — keys, strings, numbers, booleans,
null) via highlights the plugin applies directly to the buffer, with no
extra plugin or treesitter parser needed. In that split, put the cursor on a truncated header value and press `K` to see the full
value in a floating window.

The currently selected environment for the file is shown in the winbar
(`env: dev`, or `env: (none, :HttpEnv)` before you've picked one).

### Environments

`:HttpEnv` searches upward from the current file's own directory for
`http-client.env.json` (see "Directory resolution" below), and lists every
top-level key — across that file and, if present, its optional
`http-client.private.env.json` overlay — as choices:

```json
{
  "$shared": {
    "client_name": "my-app"
  },
  "dev": { "base_url": "https://dev.example.com" },
  "prod": { "base_url": "https://api.example.com" }
}
```

`"$shared"` is merged as defaults into every environment (overridden by
that environment's own values on conflict) — it's not itself a selectable
environment.

For values you don't want committed (credentials, personal tokens, or a
fully local-only environment), add an optional sibling
`http-client.private.env.json` in the same directory, same shape:

```json
{
  "$shared": {
    "api_key": "my-personal-key"
  },
  "local": { "base_url": "http://localhost:8080" }
}
```

Its values override the public file's on a per-key basis for any
environment both files define, and it may also define an environment the
public file doesn't have at all (like `local` above) — that's a valid,
selectable choice too. A missing private file is normal, not an error.
Precedence, lowest to highest: public `$shared` < public `<env>` < private
`$shared` < private `<env>`. `.gitignore` the private file (this plugin
doesn't do it for you):

```
http-client.private.env.json
```

The selected environment is remembered per env-file, so different projects
don't interfere with each other. Run `:HttpEnvVars` any time to see exactly
which values are in effect for the selected environment (the precedence
chain above, then persisted `client.global` state — see below — overriding
all of it, the same order httpfly itself applies when sending requests).
Variables added or overridden by persisted state are marked `(session)` /
`(overridden by session)`.

### Directory resolution

httpfly resolves `http-client.env.json` (plus its optional private overlay)
and `.httpfly/state.json` relative to its own process's **current working
directory only** — there's no upward directory search on httpfly's own
side. This plugin compensates by searching upward from the `.http` file's
own directory for `http-client.env.json`, and always launching httpfly with
that directory as `cwd` (falling back to the `.http` file's own directory
if no env file is found anywhere upward). This lets one
`http-client.env.json` at a project's root serve `.http` files nested
arbitrarily far below it — a subtree that needs a genuinely different
environment just keeps its own copy of the file closer to those `.http`
files, which shadows the root one for anything under it. `.httpfly/state.json`
and `.httpfly/history/` follow the same resolved directory, so persisted
session state and history are shared by every `.http` file under it, not
scoped per subdirectory.

### Chaining requests across separate sends

`client.global:set(...)` in a Lua script writes straight through to
`.httpfly/state.json`, immediately — not just at the end of a run. httpfly
does this natively, so a value one request's post-request script sets is
picked up by a later request in the same `:HttpSendAll`, and by any request
in a later, separate `:HttpSend` — no plugin hook or extra configuration
needed on this plugin's side. Run `:HttpSessionClear` to delete the state
file (e.g. once a token expires).

### Downloading files

`# @download` on a request saves its response body to disk, byte-perfect —
for any response, not just binary content (an image, a PDF, a JSON body,
plain text, ...). It's just as reliable as scripting `response.body`
yourself (see "Scripting notes" below, and httpfly's own
`6_save_response.http` example — Lua strings are byte arrays, so nothing is
lost either way, even for binary content); the advantage of `# @download`
is convenience — no script to
write, and it works the same from `:HttpSend` or `:HttpSendAll`. This is
this plugin's own annotation, not httpfly's — httpfly itself has no
per-request download marker, only a plain `-download F` flag on `run`,
which only ever applies to a single selected request. This plugin scans
the buffer for `# @download` and wires that flag up automatically whenever
it finds one, so it works the same whether you send the request with
`:HttpSend` or as part of `:HttpSendAll` (a file mixing `@download` and
ordinary requests is sent as one `httpfly run -name X` invocation per
request instead of httpfly's own single multi-request `run <file>`, so
each request can get its own `-download`; files with no `@download` at all
are unaffected and still go through one invocation).

Bare `# @download` picks a filename from the URL's last path segment.
Unlike a browser, there's no way to name the file from the *response*
(`Content-Disposition`, a content-type-guessed extension) — httpfly needs
the destination path upfront, before the request is even sent — so use
`# @download some-name.ext` to set an explicit filename whenever the URL
alone won't give you a sensible one (httpbin's `/image/png`, for instance,
has no real filename in its path). Either way, the file is saved under
`.httpfly/downloads/` next to the `.http` file — the same place
`.httpfly/history/` and `.httpfly/state.json` live, so the one `.gitignore`
entry (see below) still covers everything. That directory is created for
you if it doesn't exist yet.

The saved path shows up in the rendered output as a "Downloaded to" line in
place of the response body.

### Scripting notes

Scripts are Lua (`< {% ... %}` pre-request, `> {% ... %}` post-request) —
the only language httpfly currently supports. `response.body` is always the
raw response text, whatever its `Content-Type`; decode it yourself with
`json.decode(response.body)` when it's JSON. There's no per-request
"local variable" API for a pre-request script — the only mutable store a
script can reach is the persisted `client.global`, so even a value meant to
be used only within the current request has to go through it.

### History

Every successfully rendered response is also saved to `.httpfly/history/`
next to the `.http` file's own directory (the same directory httpfly's own
`.httpfly/state.json` lives in), named by timestamp
(`YYYYMMDD-HHMMSS-microseconds.md`).

### Gitignore

Both the persisted state file and history live under a single `.httpfly/`
directory next to your env file, so ignoring the whole thing covers both
(and the state file can hold captured secrets, so this is worth doing):

```
.httpfly/
```
