# .http File Format

httpfly reads plain-text `.http` files in the REST Client / IntelliJ HTTP
Client style: one or more requests, separated by `###`.

## Requests

```http
###
# @name Get
GET http://localhost:8080/get?greeting=hello HTTP/1.1
Accept: application/json

###
# @name Post
POST http://localhost:8080/post HTTP/1.1
Content-Type: application/json

{
  "name": "John",
  "greeting": "Hello"
}
```

A block is, in order:

1. Optional leading lines: blank lines, comments, [metadata](#metadata), and
   [local variable declarations](#variables).
2. Exactly one request line: `METHOD URL` or `METHOD URL PROTO`. `PROTO`
   defaults to `HTTP/1.1` if omitted. The URL **must be absolute**
   (`scheme://host[:port]/path`) — httpfly does not guess a scheme or
   splice in a `Host` header the way some other HTTP-file tools do; write
   the whole URL.
3. Zero or more header lines: `Name: Value`.
4. An optional blank line, then everything else in the block is the request
   body verbatim (leading/trailing blank lines trimmed; blank lines and
   indentation *within* the body are preserved).

`###` at the very start of the file is optional — content before the first
`###` (or the whole file, if there's no `###` at all) is the
[prelude](#prelude-global-vs-local), not a request.

Any line starting with `###`, anywhere in the file, starts a new block —
including one that appears inside a request body (e.g. a markdown `###
Heading` in a text body). This matches JetBrains HTTP Client's own
behavior; there's no way to escape a literal `###` inside a body. If a
body needs one, put it in a file loaded some other way rather than inline.

Trailing text on the separator line itself is shorthand for that block's
`@name`: `### GetUsers` is equivalent to a bare `###` followed by
`# @name GetUsers`. An explicit `# @name` line later in the same block is
fine as long as it matches; if it names something *different*, that's an
error (`metadata:name`) rather than one silently overwriting the other.

## Metadata

`# @key value` comment lines attach metadata to a request:

| Key | Required | Scope | Meaning |
|---|---|---|---|
| `name` | **Yes** | request only | The request's identifier — used by `-name` on the CLI. Every request must have a non-empty `@name`, supplied either as `# @name Value` or as trailing text on the block's `###` separator line (see [above](#requests)); it's an error at file-prelude scope (there's no such thing as a "global name"). |
| `lang` | No | request only | Names the scripting language for pre-/post-request scripts. `lua` is the only supported value (and the default if omitted); anything else is a warning and falls back to Lua. |
| `proxy` | No | request or global | An absolute proxy URL (`scheme://host[:port]`) to send this request through. See [below](#proxy). |

Any other `@key` is accepted but reported as an "unknown metadata" warning
— it's recognized syntax, just not something httpfly currently acts on.

### `proxy`

```http
# @proxy http://localhost:3128

###
# @name Get
GET http://localhost:8080/get HTTP/1.1

###
# @name GetViaOtherProxy
# @proxy http://localhost:9999
GET http://localhost:8080/get HTTP/1.1
```

Declared in the [prelude](#prelude-global-vs-local), `@proxy` becomes the
default for every request in the file. Declared inside a block, it
overrides that default for just that one request. The value must resolve
(after variable interpolation) to an absolute URL — `localhost:3128` is
rejected as ambiguous; write `http://localhost:3128`.

There's currently no way to opt a single request *out* of a global proxy
back to a direct connection — `# @proxy` with no value is a validation
error, not "no proxy." If you need that, don't declare `@proxy` globally;
add it to each request that needs it instead.

## Variables

`{{name}}` anywhere in the URL, a header value, or the body is replaced
with a variable's value before the request is sent. `@key = value` lines
(not to be confused with `# @key value` metadata, which is `#`-prefixed)
declare variables:

```http
@host = http://localhost:8080
@greeting = hello

###
# @name Get
GET {{host}}/get?greeting={{greeting}} HTTP/1.1
```

### Prelude: global vs. local

An `@key = value` line before the first `###` declares a **global**
variable — the default for every request in the file. The same kind of
line *inside* a request block declares a variable **local** to that block
only, overriding the global value just for that one request (it never
leaks into other blocks):

```http
@env = prod

###
# @name UsesGlobal
GET http://localhost:8080/get?env={{env}} HTTP/1.1   # -> env=prod

###
# @name UsesLocal
@env = dev
GET http://localhost:8080/get?env={{env}} HTTP/1.1   # -> env=dev
```

Precedence, highest to lowest: **local variable > persisted `client.global`
(scripting) > `-env` environment variable > global (prelude) variable**.
See [Environments](environments.md) and [Scripting](scripting.md) for the
middle two tiers.

An `{{unknown}}` placeholder with no matching declaration is left as
literal text in the output and reported as a warning — it doesn't stop the
request from being parsed or sent.

### URL encoding

A variable substituted into a query parameter's *value* is percent-encoded
automatically, so a value containing a space or `&` doesn't corrupt the
request:

```http
@greeting = Hello again

GET http://localhost:8080/get?greeting={{greeting}} HTTP/1.1
# sent as: ?greeting=Hello+again
```

Anywhere else in the URL — the host/scheme prefix, or a path segment — a
variable is substituted raw, so `{{host}}` holding a full
`http://localhost:8080` prefix works as expected instead of having its `/`
and `:` mangled into `%2F`/`%3A`.

## File uploads

A body line reading `< path/to/file` is replaced with that file's raw
bytes (JetBrains HTTP Client's own convention) — no pre-request script
needed to read a file in by hand:

```http
### RawUpload
PUT http://localhost:8080/put HTTP/1.1
Content-Type: image/png

< ./photo.png
```

The same line shape works inside one part of a multipart body:

```http
### MultipartUpload
POST http://localhost:8080/post HTTP/1.1
Content-Type: multipart/form-data; boundary=WebAppBoundary

--WebAppBoundary
Content-Disposition: form-data; name="avatar"; filename="photo.png"
Content-Type: image/png

< ./photo.png
--WebAppBoundary--
```

A relative path resolves against the current working directory — same
rule as everything else httpfly reads from disk (see
[Environments](environments.md)), not the `.http` file's own directory.
The path can itself use `{{var}}` interpolation (e.g. `< ./{{env}}/cert.pem`).

A file that can't be read is a warning at `validate` time (a pre-request
script might still create it before the request is actually sent — same
reasoning as an undefined `{{variable}}`), escalated to a hard error by
`run`/`convert to-curl` immediately before it's actually needed.

`convert from-curl`/`convert to-curl` understand this too — see
[Usage](usage.md#convert-from-curl).

## Comments

Any `#`-prefixed line that isn't `# @key value` metadata is a plain
comment and is ignored.

## Validation

`httpfly validate` checks every block (see [Usage](usage.md#validate) for
the report format). At a glance:

| Severity | Examples |
|---|---|
| `ERROR` (blocks the request / fails validation) | Missing or empty `@name`; the same `@name` used by more than one request in the file; a separator-line name that conflicts with an explicit `# @name` in the same block; request line that isn't `METHOD URL [PROTO]`; relative URL; malformed header line (no `:`); `@proxy` that isn't absolute; `@name`/`@lang` declared in the prelude; a malformed or unterminated script block; a script with invalid Lua syntax. |
| `WARN` (still usable, just worth knowing) | Unknown `@key`; non-standard HTTP method; unrecognized `PROTO` string; `@lang` set to something other than `lua`; undefined `{{variable}}`; a `< path/to/file` body reference that couldn't be read. |

`doc/examples/invalid.http` in this repo demonstrates each of these, one
issue per block — a good file to run `httpfly validate` against to see the
report format.

## Scripting

A request can carry a `< {% ... %}` (pre-request) and/or `> {% ... %}`
(post-request) Lua script — for capturing an auth token from a response,
reading a local file, or running a command. See [Scripting](scripting.md)
for the full reference; `httpfly validate` only checks a script compiles,
it never executes one.
