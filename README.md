# httpfly.nvim

Send requests from JetBrains-style `.http` files in Neovim and view the
responses in a colored result pane.

![httpfly.nvim showing a request file next to the rendered response](screenshot.png)

- `###`-separated requests with `{{variables}}`, dynamic values
  (`{{$uuid}}`, `{{$timestamp}}`, ...), proxies and file uploads
- Environments from `http-client.env.json`, with an optional
  uncommitted `http-client.private.env.json`
- Lua pre-/post-request scripts, with variables that persist across sends
  (e.g. a login token)
- Headers as tables, and JSON bodies pretty-printed and highlighted
- Response bodies saved to disk with `# @download`
- A saved copy of every result

Requests are sent by httpfly, a small Go program in [`backend/`](backend/)
that is compiled when the plugin is installed.

## Requirements

- Neovim 0.10+
- Go 1.26+ and `make`, to build the backend

## Installation

With [lazy.nvim](https://github.com/folke/lazy.nvim):

```lua
{
  "cristianradulescu/httpfly.nvim",
  ft = "http",
  build = "make build",
  opts = {},
}
```

`make build` compiles the backend into the plugin's own `bin/httpfly`,
and lazy.nvim reruns it on every update. With other plugin managers, run
`make build` in the plugin directory after installing or updating.
`:checkhealth httpfly` shows whether the backend is built.

## Configuration

All options are optional. These are the defaults:

```lua
require("httpfly").setup({
  env_file = "http-client.env.json",                 -- environment file name
  private_env_file = "http-client.private.env.json", -- private overlay, next to env_file
  keymaps = true,                                    -- set the <leader>h* keymaps below
  max_header_value_len = 100,                        -- truncate longer header values in tables
  preview_keymap = "K",                              -- show a truncated value in full
  timeout = nil,                                     -- per-request timeout, e.g. "2m"; nil = 30s
})
```

## Usage

In a `.http` file:

| Command             | Keymap       | Does                                              |
|---------------------|--------------|---------------------------------------------------|
| `:HttpSend`         | `<leader>hs` | Send the request under the cursor                 |
| `:HttpSendAll`      | `<leader>ha` | Send every request in the file                    |
| `:HttpEnv`          | `<leader>he` | Pick the environment                              |
| `:HttpEnv dev`      |              | Select an environment directly                    |
| `:HttpEnvVars`      | `<leader>hv` | Show the resolved variables                       |
| `:HttpSessionClear` | `<leader>hc` | Delete variables persisted by scripts             |

A request looks like this (every request needs a name):

```http
@host = http://localhost:8080

### GetUser
GET {{host}}/users/1 HTTP/1.1
Accept: application/json

### CreateUser
POST {{host}}/users HTTP/1.1
Content-Type: application/json

{"name": "Ada"}
```

The result opens in a vertical split. Put the cursor on a truncated header
value and press `K` to see all of it. The winbar shows the selected
environment.

### Downloading files

Add `# @download` to a request to save its response body, byte for byte,
under `.httpfly/downloads/` instead of showing it. The file is named after
the URL's last path segment; use `# @download name.ext` to choose the
name. The result pane shows the saved path.

```http
### GetLogo
# @download logo.png
GET https://example.com/logo.png HTTP/1.1
```

### The `.httpfly/` directory

A `.httpfly/` directory is created next to `http-client.env.json` (or
next to the `.http` file if there's no environment file). It holds:

- `state.json`: variables persisted by scripts, which may be secrets
- `history/`: a copy of every rendered result
- `downloads/`: `# @download` files

Add it to your `.gitignore`:

```
.httpfly/
```

## Documentation

- [.http file format](docs/http-file-format.md): requests, metadata,
  variables, file uploads
- [Environments](docs/environments.md): `http-client.env.json`, private
  overrides, and how the file is found
- [Scripting](docs/scripting.md): Lua pre-/post-request scripts and
  persisted variables
- [Examples](docs/examples/): runnable `.http` files. Run
  `make httpbin-up` first for a local test server (needs Docker).
- [Backend](docs/backend.md): how the plugin runs httpfly, its command
  line and its JSON output

## Development

The Lua plugin is at the repo root (`lua/`, `plugin/`, `ftdetect/`,
`ftplugin/`) and the Go backend is in `backend/`. The `Makefile` covers
both:

```sh
make build        # compile the backend into bin/httpfly
make check        # gofmt check, go vet, go test, stylua --check
make httpbin-up   # local httpbin on :8080 for docs/examples
make httpbin-down
```
