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

## Metadata

`# @key value` comment lines attach metadata to a request:

| Key | Required | Scope | Meaning |
|---|---|---|---|
| `name` | **Yes** | request only | The request's identifier — used by `-name` on the CLI. Every request must have a non-empty `@name`; it's an error at file-prelude scope (there's no such thing as a "global name"). |
| `lang` | No | request only | Names the scripting language for pre-/post-request scripts. Recognized, but scripting itself (`< {% %}` / `> {% %}`) isn't implemented yet — using it is always reported as a warning (and a script block is an error). |
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
with a variable's value before the request is sent. `key = value` lines
(no `@`, no `#` — that prefix is reserved for metadata) declare variables:

```http
host = http://localhost:8080
greeting = hello

###
# @name Get
GET {{host}}/get?greeting={{greeting}} HTTP/1.1
```

### Prelude: global vs. local

A `key = value` line before the first `###` declares a **global**
variable — the default for every request in the file. The same kind of
line *inside* a request block declares a variable **local** to that block
only, overriding the global value just for that one request (it never
leaks into other blocks):

```http
env = prod

###
# @name UsesGlobal
GET http://localhost:8080/get?env={{env}} HTTP/1.1   # -> env=prod

###
# @name UsesLocal
env = dev
GET http://localhost:8080/get?env={{env}} HTTP/1.1   # -> env=dev
```

Precedence, highest to lowest: **local variable > `-env` environment
variable > global (prelude) variable**. See [Environments](environments.md)
for the middle tier.

An `{{unknown}}` placeholder with no matching declaration is left as
literal text in the output and reported as a warning — it doesn't stop the
request from being parsed or sent.

### URL encoding

A variable substituted into a query parameter's *value* is percent-encoded
automatically, so a value containing a space or `&` doesn't corrupt the
request:

```http
greeting = Hello again

GET http://localhost:8080/get?greeting={{greeting}} HTTP/1.1
# sent as: ?greeting=Hello+again
```

Anywhere else in the URL — the host/scheme prefix, or a path segment — a
variable is substituted raw, so `{{host}}` holding a full
`http://localhost:8080` prefix works as expected instead of having its `/`
and `:` mangled into `%2F`/`%3A`.

## Comments

Any `#`-prefixed line that isn't `# @key value` metadata is a plain
comment and is ignored.

## Validation

`httpfly validate` checks every block (see [Usage](usage.md#validate) for
the report format). At a glance:

| Severity | Examples |
|---|---|
| `ERROR` (blocks the request / fails validation) | Missing or empty `@name`; request line that isn't `METHOD URL [PROTO]`; relative URL; malformed header line (no `:`); `@proxy` that isn't absolute; a scripting block (`< {%`/`> {%`); `@name`/`@lang` declared in the prelude. |
| `WARN` (still usable, just worth knowing) | Unknown `@key`; non-standard HTTP method; unrecognized `PROTO` string; `@lang` (scripting isn't implemented); undefined `{{variable}}`. |

`doc/examples/invalid.http` in this repo demonstrates each of these, one
issue per block — a good file to run `httpfly validate` against to see the
report format.

## Not yet implemented

Pre-/post-request scripting (`< {% ... %}` before the request line, `>
{% ... %}` after it) is recognized syntactically — httpfly's validator
correctly identifies these blocks — but isn't executed. A file containing
one will report an error from `validate` and refuse to `run`.
