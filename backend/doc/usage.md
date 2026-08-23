# Usage

```
httpfly <command> [arguments]
```

## Commands

### `run`

```
httpfly run [-name X] [-env E] [-s | -json] [-v] <file.http>
```

Parses `<file.http>`, sends every request it defines (in file order, or
just one with `-name`), and prints the result of each.

Exit code is non-zero only when a request fails to *send* (DNS failure,
connection refused, timeout, ...), its post-request script errors, or the
file fails to parse. A non-2xx HTTP response (404, 500, ...) is a normal
result, not a failure — it's printed like any other response and doesn't
affect the exit code.

In plain-text/`-s` mode, a failure also prints a one-line summary
(`run: N of M request(s) failed`) after all results. **`-json` mode never
prints this or anything else beyond the JSON array** — each failed
request's own `error`/`script_error` field already carries that
information, and the exit code alone signals overall failure, so a tool
reading the array (even one that merges stdout and stderr) never has to
deal with trailing non-JSON text.

A `< {% %}`/`> {% %}` script attached to a request runs right before it's
sent / right after its response arrives — see [Scripting](scripting.md).

**Plain-text output** (the default), one block per request:

```
=== <name> ===
<METHOD> <URL> <PROTO>
<request headers>

<request body, if any>

[redirected to: <final URL>, if the response came after a redirect]
<status> (<duration>)
<response headers>

[tls: ..., if -v]

<response body, if any>
```

**`-json`** switches to a JSON array instead — see [below](#-json).

### `validate`

```
httpfly validate [-name X] [-env E] <file.http>
```

Parses `<file.http>` and reports every issue found, without sending
anything. Exit code is non-zero if any block (or the file's prelude) has an
`ERROR`-severity issue; `WARN`-severity issues don't affect the exit code.
See [.http File Format](http-file-format.md#validation) for what gets
checked and at which severity.

```
<file>: <N> global variable(s) declared: <names>

<file> (global):
  [ERROR|WARN] <element>: <message>

<file> (<request name>):
  ok
  [ERROR|WARN] <element>: <message>

<N> request(s) checked, <N> error(s), <N> warning(s)
```

### `convert from-curl`

```
httpfly convert from-curl [-name X] [-file F]
```

Converts a **bash-style** `curl` command (e.g. a browser's "Copy as cURL"
output) into a single `.http` request block, printed to stdout. Reads the
curl command from stdin by default, or from a file with `-file`:

```sh
pbpaste | httpfly convert from-curl -name GetUser > get-user.http
httpfly convert from-curl -file request.curl -name GetUser > get-user.http
```

`-name` sets the generated request's `@name` (mandatory in the `.http`
format); if omitted, it defaults to `ConvertedRequest`.

Handles both `curl <url> ...` and `curl --url <url> ...`, multi-line
commands with `\` line continuations, and single- or double-quoted
arguments (including headers with embedded quotes, colons, or commas —
common in real captured requests, e.g. `sec-ch-ua`, `if-none-match`).
`-H`/`--header` become header lines; `-b`/`--cookie` becomes a `Cookie`
header; `-u`/`--user` becomes a Basic `Authorization` header; `-x`/`--proxy`
becomes `@proxy`; `-d`/`--data*` becomes the body (and infers `POST` if no
explicit `-X` is given, matching curl's own rule). A flag with no httpfly
equivalent (`-k`/`--insecure`, `-o`/`--output`, reading data from a file
with `@file`) is dropped with a warning printed to stderr — the generated
`.http` file on stdout is never polluted by these warnings.

Only bash-style curl output is supported — `cmd.exe`/PowerShell "Copy as
cURL" variants use different escaping and aren't handled.

**The generated file is exactly as sensitive as the curl command it came
from** — a command copied from an authenticated browser tab will carry
live session cookies/tokens in plaintext. Treat it with the same care
you'd give the original curl command (don't commit it, etc.) — httpfly
doesn't do anything special to protect or flag that content.

### `convert to-curl`

```
httpfly convert to-curl [-name X] [-env E] <file.http>
```

The reverse: converts one request from `<file.http>` into a multi-line,
bash-style curl command, printed to stdout — for pasting into a terminal
or sharing with someone who doesn't use httpfly.

`-name` picks which request to convert; required if the file has more
than one (there's no default when it's ambiguous). `-env` resolves
`{{variable}}` placeholders the same way `run`/`validate` do (prelude →
`-env` environment → persisted `client.global` state → the request's own
local variables) — but **no script runs** as part of this conversion
(same read-only principle as `validate`), and a variable still undefined
after that resolution is a **hard error**, not a warning: a curl command
containing a literal `{{name}}` would just be broken, and unlike `run`
there's no later chance for a script to fill it in.

`Method` is always emitted explicitly via `-X` (even for `GET`), headers
become `-H 'Name: Value'` in their original order, `Body` becomes
`--data-raw`, and `@proxy` becomes `-x`. A `Cookie` or Basic-auth
`Authorization` header is emitted as a plain header, not un-mapped back
into `-b`/`-u` — equally valid curl, without guessing intent.

```sh
httpfly convert to-curl -name Login doc/examples/3_scripting.http
```

### `help`

```
httpfly help
```

Same as `-h`/`--help`, or running httpfly with no arguments (which also
exits non-zero, since a command is required).

## Flags

| Flag | Commands | Meaning |
|---|---|---|
| `-name X` | `run`, `validate` | Restrict to the single request declared with `# @name X`. Errors if no request has that name. |
| `-name X` | `convert from-curl` | Sets the generated request's `@name` (different meaning than for `run`/`validate` — there's no existing request to restrict to). Defaults to `ConvertedRequest` if omitted. |
| `-name X` | `convert to-curl` | Picks which request to convert. Required if the file has more than one. |
| `-env E` | `run`, `validate`, `convert to-curl` | Apply variables from the environment named `E` in `httpfly.env.json`, found in the current working directory (not `<file.http>`'s own directory). See [Environments](environments.md). |
| `-s`, `-silent` | `run` | Print only response bodies, back to back, nothing else — like `curl -s`. A failed request prints nothing for itself but still counts toward the exit code. Mutually exclusive with `-json`. |
| `-json` | `run` | Print a JSON array instead of plain text. See below. Mutually exclusive with `-s`/`-silent`. |
| `-v`, `-verbose` | `run` | Also report TLS connection details (version, cipher suite, ALPN protocol, peer certificate). No effect on a plain HTTP (non-TLS) request. |
| `-file F` | `convert from-curl` | Read the curl command from file `F` instead of stdin. |

`-s`/`-silent` and `-v`/`-verbose` are two names for the same flag — use
either.

## `-json`

Each request becomes one object in a JSON array:

```json
[
  {
    "name": "GetExample",
    "request": {
      "method": "GET",
      "url": "https://example.com/get?greeting=hi",
      "proto": "HTTP/1.1",
      "headers": { "Accept": ["application/json"] },
      "body": ""
    },
    "response": {
      "status_code": 200,
      "headers": { "Content-Type": ["application/json"] },
      "body": "{...}",
      "tls": { "...": "... (only present with -v)" }
    },
    "final_url": "... (only present if the request was redirected)",
    "script_error": "... (only present if a post-request script errored)",
    "duration_ms": 12
  }
]
```

Headers are always `name -> array of values` (never a bare string), since a
header name can legitimately repeat (`Set-Cookie` being the common case).

If a request fails to send, its object has an `"error"` string instead of
`"response"`:

```json
{
  "name": "Unreachable",
  "request": { "...": "..." },
  "error": "Get \"http://127.0.0.1:1/get\": dial tcp 127.0.0.1:1: connect: connection refused",
  "duration_ms": 0
}
```

With `-v`, `response.tls` includes the *full* peer certificate chain (every
certificate, not just the leaf — the plain-text output only prints the
leaf, for brevity).

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success. |
| `1` | Bad arguments, a file that failed to parse, `validate` finding an `ERROR`-severity issue, or `run` failing to send one or more requests (including a pre-request script error) or a post-request script erroring. |
