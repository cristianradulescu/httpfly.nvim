# httpfly.nvim

Send `.http` requests from Neovim using [httpyac](https://httpyac.github.io/)
and view the response as readable markdown.

![httpfly.nvim showing a request file next to the rendered response](screenshot.png)

> **New here?** Check out `doc/examples/` for runnable `.http` files
> covering everything below, from basic requests to scripting.

## Scope

This plugin does **not** implement request execution, variables, or
scripting itself — [httpyac](https://httpyac.github.io/) does that. This
plugin just wires it into Neovim:

- discovers/selects the httpyac environment (`http-client.env.json`) for the
  current file
- runs `httpyac send` on the request under your cursor (or the whole file)
- formats the JSON result as markdown (request/response headers, bodies
  pretty-printed when JSON, status line) in a split
- lets you preview values that got truncated in header tables (e.g. long
  bearer tokens) in a floating window
- saves a copy of every result under `.httpfly/history/`

If you need JetBrains HTTP Client syntax support (`###` separators,
`{{variables}}`, `> {% ... %}` pre/post-request scripts,
`http-client.private.env.json`), that comes from httpyac — this plugin
doesn't reimplement or restrict any of it.

See `doc/examples/` for runnable `.http` files: `1_basic.http` (plain
GET/POST), `2_scripting.http` (pre-/post-request scripting, including a
login → token → authenticated-request chain), `3_global_headers.http`
(a `{{@request ... }}` block applying a header, e.g. a custom User-Agent,
to every request in the file), `4_debugging.http` (using
`client.test(...)` to dump values into the rendered output, since
`console.log` is silently dropped — see "Scripting notes" below), and
`5_environments.http` (uses `{{base_url}}`/`{{client_name}}` from the
`http-client.env.json` in that same directory — pick an environment with
`:HttpEnv` first), `6_shell_auth.http` (a pre-request script shelling out
to `generate-token.sh` and using its stdout as the request's token — for
auth flows too complex to reimplement inline), `7_forms.http`
(`application/x-www-form-urlencoded`, `multipart/form-data`, and a
multipart file upload via `< ./path`), and `8_download.http` (a
post-request script saving a JSON/text response body to `/tmp` —
including a note on why saving genuinely binary content this way doesn't
work reliably). They hit a local httpbin instance — run `make httpbin-up`
first (requires Docker), `make httpbin-down` when done.

## Requirements

- Neovim 0.10+
- [httpyac](https://httpyac.github.io/) on your `$PATH`:
  ```sh
  npm install -g httpyac
  ```

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
  cmd = "httpyac",              -- httpyac binary/command to run
  env_file = "http-client.env.json", -- environment file name to look for
  keymaps = true,                -- set the default <leader>h* keymaps below
  max_header_value_len = 100,    -- header table cell truncation length
  preview_keymap = "K",          -- keymap to preview a truncated value
  output_style = "markdown",     -- "markdown" or "unicode"
})
```

## Usage

Open a `.http` file and:

| Command         | Keymap        | Does                                                |
|-----------------|---------------|------------------------------------------------------|
| `:HttpEnv`      | `<leader>he`  | Pick the httpyac environment for this file            |
| `:HttpEnv dev`  | —             | Set the environment directly, without the picker      |
| `:HttpEnvVars`  | `<leader>hv`  | Show the selected environment's merged variables       |
| `:HttpSend`     | `<leader>hs`  | Send the request under the cursor                     |
| `:HttpSendAll`  | `<leader>ha`  | Send every request in the file                        |
| `:HttpSessionClear` | `<leader>hc` | Clear the session file (see below)                |

The response opens in a vertical split as markdown (`filetype = "markdown"`).
Set `output_style = "unicode"` for the same layout (headers as a table,
request/response sections, a body block) rendered with Unicode box-drawing
characters (`┌─┬─┐`, `━━━`) instead of markdown syntax — no `**bold**`,
`` ``` `` fences, or `|---|` pipes, `filetype = "text"`. It's colored too
(status/method/section titles/JSON body syntax — keys, strings, numbers,
booleans, null/etc.), via highlights the plugin applies directly to the
buffer — no markdown-rendering plugin or treesitter parser needed. Useful
if you don't want a markdown-rendering plugin touching this
buffer, or just prefer that look. History files (see below) get `.txt`
instead of `.md` to match. In that split, put the
cursor on a truncated header value and press `K` to see the full value in a
floating window (works the same in both styles).

The currently selected environment for the file is shown in the winbar
(`env: dev`, or `env: (none, :HttpEnv)` before you've picked one).

### Environments

`:HttpEnv` looks for the nearest `http-client.env.json` by walking up from
the current file's directory, and lists its (and its private file's, see
below — an environment defined only there still shows up) top-level keys
as choices — the same file format used by IntelliJ/WebStorm's HTTP Client,
so an existing one works as-is:

```json
{
  "dev": { "base_url": "https://dev.example.com" },
  "prod": { "base_url": "https://api.example.com" }
}
```

Secrets go in a sibling `http-client.private.env.json` (gitignore it);
httpyac merges both by environment name automatically, with the private
file's values winning on conflicts. A top-level `"$shared"` key in either
file is merged into every environment, also matching the IntelliJ format.

The selected environment is remembered per env-file, so different projects
don't interfere with each other. Run `:HttpEnvVars` any time to see exactly
which values are in effect for the selected environment (shared + env,
shared file then private file, then session variables — see below —
overriding those, the same order httpyac itself applies when sending
requests). Variables added or overridden by the session are marked
`[session]` / `[overridden by session]`.

### Chaining requests across separate sends

`client.global.set(...)` in a script only lives for the duration of one
`httpyac` process by default, so a variable set by one request's script
would normally be gone by the time you `:HttpSend` a later request as a
separate invocation — even though it works fine within a single
`:HttpSendAll` (one process for the whole file). This plugin makes that
"just work" automatically: it points httpyac at a small bundled plugin
(`httpyac-plugin/session-persist.js`, loaded via the `HTTPYAC_PLUGIN` env
var on every send) that mirrors httpyac's own global-variable cache to
`.httpfly/session.json` under your current working directory — the same
place `.httpfly/history/` lives (see below), regardless of where your
`.http` file or env file are. No changes to your `.http` files or scripts
are needed — write `client.global.set(...)` exactly as you already do; it
now survives across separate `:HttpSend` calls, and `{{your_var}}`
resolves correctly in later requests without ever touching
`http-client.env.json`.

Run `:HttpSessionClear` to delete the session file (e.g. once a token
expires).

### Scripting notes

This plugin always runs httpyac with `--json` (that's what lets it render
the markdown response view), and httpyac's `--json` mode only ever writes a
single JSON blob to stdout — any `console.log` / `console.info` /
`console.warn` calls in your `> {% ... %}` scripts are silently dropped,
not shown anywhere, even though they work fine when running httpyac
directly without `--json`. There's no way to recover that output through
this plugin.

If you want a script to leave a visible marker in the rendered output, use
a test assertion instead — those *do* survive `--json` and are rendered as
a "Test Results" section:

```js
> {%
  client.test("I AM LOGGED", () => true);
%}
```

### History

Every successfully rendered response is also saved to `.httpfly/history/`
in your current working directory, named by timestamp
(`YYYYMMDD-HHMMSS-microseconds.md`).

### Gitignore

Both the session file and history live under a single `.httpfly/`
directory next to your env file, so ignoring the whole thing covers both
(and the session file can hold captured secrets, so this is worth doing):

```
.httpfly/
```
