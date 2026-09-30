# Scripting

A request can carry a Lua script that runs just before it's sent
(**pre-request**) and/or just after its response arrives
(**post-request**) — for capturing a token from a login response, reading
a local file into a variable, or shelling out to compute something.

## Syntax

```http
###
# @name Login
GET http://localhost:8080/uuid HTTP/1.1

> {%
  local json = require("json")
  local data = json.decode(response.body)
  client.global:set("auth_token", data.uuid)
%}
```

- `< {% ... %}` — a **pre-request** script, placed before the request line.
- `> {% ... %}` — a **post-request** script, placed after the headers/body.
- The opening line must be exactly `< {%` or `> {%`; the block ends at a
  line that's exactly `%}`. Everything in between is passed to Lua as-is
  (indentation preserved).
- `# @lang lua` is implied and doesn't need to be written; any other value
  is a warning and falls back to Lua, since it's the only language
  currently supported.

A Lua syntax error is reported before anything in the file is sent.

## Object model

| Global | Available in | Meaning |
|---|---|---|
| `client.global:get(name)` / `client.global:set(name, value)` | pre- and post-request | Persisted variables — see [Persistence](#persistence) below. `get` returns `nil` for a variable that was never set (so `client.global:get("token") or "default"` works as expected); a value is always a string, `set` requires one. |
| `response.status` | post-request only | The HTTP status code, e.g. `200`. |
| `response.headers` | post-request only | A table of header name → array of values (a header can repeat, e.g. `Set-Cookie`). |
| `response.body` | post-request only | The raw response body as a string. Parse it with `json.decode(response.body)` (see below) if it's JSON. |

## File reads and running commands

Four modules from [gopher-lua-libs](https://github.com/vadv/gopher-lua-libs)
are preloaded — `require` them like any Lua module:

```lua
local json = require("json")
local ioutil = require("ioutil")
local filepath = require("filepath")
local cmd = require("cmd")

local content = ioutil.read_file("/path/to/file")
local result = cmd.exec("date +%s")        -- {status, stdout, stderr}
local data = json.decode(response.body)
local encoded = json.encode({name = "Ada"})
```

See [`examples/6_shell_auth.http`](examples/6_shell_auth.http) for
`cmd.exec` (shelling out to a script for a token) and
[`examples/7_save_response.http`](examples/7_save_response.http) for
`ioutil.write_file` (saving a response body yourself). Relative paths
resolve from the directory holding `http-client.env.json` (see
[Environments](environments.md#where-its-found)).

Scripts run with full trust, with the same access to the filesystem and
commands as Neovim itself. There is no sandbox. You're running
your own `.http` files; treat a script in one the same way you'd treat a
shell script you wrote yourself. See each module's own documentation for
its full API (`cmd.exec`, `ioutil.read_file`/`write_file`/`copy`,
`filepath.*`, `json.decode`/`encode`).

## Debug output

`print(...)` output appears in a separate `stderr` section of the result
pane.

## Persistence

`client.global:set(...)` writes to `.httpfly/state.json` **immediately**,
not at the end of the send. Every request resolves its `{{var}}`
placeholders right before it's sent, using whatever `client.global` holds
at that moment. So a value set by one request's post-request script is
picked up by:

- a later request in the same `:HttpSendAll`, and
- any request in a later `:HttpSend`.

That's how the login → token → authenticated-request pattern works,
whether you send the whole file at once or log in once and then send the
authenticated request as often as you need.

The state file sits next to `http-client.env.json` (see
[Environments](environments.md#where-its-found)), so every `.http` file
under that directory shares it. Values are kept separately per
environment: `dev` and `prod` never share values, and "no environment"
has its own set. `:HttpEnvVars` shows the persisted values and
`:HttpSessionClear` deletes the file, e.g. once a token has expired. Add
`.httpfly/` to your `.gitignore`, because the file is likely to hold real
secrets.

### If a variable is still undefined right before sending

A request that still has an undefined `{{var}}` right before sending
isn't sent. The result pane shows an error for it instead, so a literal
`{{name}}` never goes out on the wire.

## Precedence

Persisted `client.global` variables sit between the environment and a
request's own local variables:

**local variable > persisted `client.global` > selected environment > file's global (prelude)**

See [.http File Format](http-file-format.md#variables) and
[Environments](environments.md#precedence) for the other tiers.

## Example

[`examples/3_scripting.http`](examples/3_scripting.http) is a working
login → token → authenticated-request chain against the local httpbin
container (`make httpbin-up`). `:HttpSendAll` sends both requests. You can
also `:HttpSend` on `Login` once and then send `AuthenticatedRequest` on
its own as many times as you like.
