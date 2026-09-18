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

`httpfly validate` compiles (but never runs) a script's Lua source, so
syntax errors are caught without any of the side effects actually running
it might have — same principle as `validate` never sending a network
request.

## Object model

| Global | Available in | Meaning |
|---|---|---|
| `client.global:get(name)` / `client.global:set(name, value)` | pre- and post-request | Persisted variables — see [Persistence](#persistence) below. |
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

See [`doc/examples/6_shell_auth.http`](examples/6_shell_auth.http) for
`cmd.exec` (shelling out to a script for a token) and
[`doc/examples/7_save_response.http`](examples/7_save_response.http) for
`ioutil.write_file` (saving a response body yourself).

Scripts run with full trust — the same access to the filesystem and to run
commands that the `httpfly` process itself has, no sandbox. You're running
your own `.http` files; treat a script in one the same way you'd treat a
shell script you wrote yourself. See each module's own documentation for
its full API (`cmd.exec`, `ioutil.read_file`/`write_file`/`copy`,
`filepath.*`, `json.decode`/`encode`).

## Debug output

`print(...)` works as usual but writes to **stderr**, not stdout — so a
`httpfly run -json` invocation's stdout stays a single clean JSON document
even if a script prints while debugging.

## Persistence

`client.global:set(...)` writes through to `.httpfly/state.json`,
**immediately** — not just at the end of the run. Every request resolves
its `{{var}}` placeholders right before it's sent, using whatever is
currently in `client.global` at that moment — so a value one request's
post-request script sets is picked up by:

- a **later request in the same `httpfly run`**, and
- any request in a **later, separate `httpfly run`** invocation, run from
  the same working directory.

That's what makes the login → token → authenticated-request pattern work
either running the whole file in one command, or running each request
separately (e.g. because you only want to re-authenticate occasionally,
not on every run) — see the example below.

State lives in `.httpfly/state.json` **in the current working
directory** — wherever you launch `httpfly` from, not the directory
containing the `.http` file (same rule as `http-client.env.json`; see
[Environments](environments.md)) — scoped per **environment** (`-env dev`
and `-env prod` never share values; no `-env` gets its own bucket too).
This means several `.http` files in different subdirectories can share
one `client.global` state as long as you run httpfly from their common
parent. Add `.httpfly/` to your `.gitignore` — it's very likely to hold
real secrets like tokens.

### If a variable is still undefined right before sending

`validate` only ever warns about an undefined `{{var}}` (a script that
hasn't run yet might still set it). `run` re-checks each request
immediately before actually sending it — at that point there's no "might"
left, so a still-undefined variable there is a hard error: the request
isn't sent (avoiding silently sending a literal `{{name}}`), and it counts
toward `run`'s failure count and exit code.

## Precedence

Persisted `client.global` variables sit between the environment and a
request's own local variables:

**local variable > persisted `client.global` > `-env` environment > file's global (prelude)**

See [.http File Format](http-file-format.md#variables) and
[Environments](environments.md#precedence) for the other tiers.

## Example

[`doc/examples/3_scripting.http`](examples/3_scripting.http) — a working
login → token → authenticated-request chain against the local httpbin
container. Both work:

```sh
make httpbin-up
cd doc/examples   # .httpfly/state.json is created here, in the CWD

# the whole file in one command -- AuthenticatedRequest sees the token
# Login's post-request script just set, in the same run
httpfly run 3_scripting.http

# or one request at a time, in separate commands -- AuthenticatedRequest
# picks up whatever Login set the last time IT ran, from .httpfly/state.json
httpfly run -name Login 3_scripting.http
httpfly run -name AuthenticatedRequest 3_scripting.http
```
