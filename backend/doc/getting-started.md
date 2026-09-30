# Getting Started

This walks through writing and running your first `.http` file. See
[Installation](installation.md) first if you haven't built httpfly yet.

## 1. Write a request

Create a file named `example.http`:

```http
###
# @name GetExample
GET https://example.com/ HTTP/1.1
Accept: text/html
```

Every request needs a name — it's the identifier httpfly (and you) use to
refer to that request, e.g. with `-name`. `# @name GetExample` above works,
or you can fold it into the separator line instead: `### GetExample`.

## 2. Validate it first

Before sending anything, check the file is well-formed:

```sh
httpfly validate example.http
```

```
example.http (GetExample):
  ok

1 request(s) checked, 0 error(s), 0 warning(s)
```

`validate` never makes a network call — it's safe to run on any file,
anytime. See [.http File Format](http-file-format.md) for what it checks.

## 3. Run it

```sh
httpfly run example.http
```

```
=== GetExample ===
GET https://example.com/ HTTP/1.1
Accept: text/html

200 OK (123ms)
Content-Type: text/html
...

<!doctype html>...
```

## 4. Try the bundled examples

The repo ships a handful of example `.http` files under `doc/examples/`
that exercise most of what httpfly can do. Some of them talk to a local
[httpbin](https://httpbin.org/) instance instead of the internet, so you
don't need network access or an account anywhere:

```sh
make httpbin-up      # starts a local httpbin container on :8080 (needs Docker)

httpfly run doc/examples/1_basic.http        # plain GET/POST requests
httpfly run doc/examples/2_variables.http    # {{variable}} interpolation
httpfly validate doc/examples/invalid.http   # one broken request per validation rule

make httpbin-down    # stop it when you're done
```

## 5. A request with a body and a variable

```http
###
# @name CreateUser
POST http://localhost:8080/post HTTP/1.1
Content-Type: application/json

{
  "name": "Ada"
}
```

Add a variable so the host isn't hardcoded three times over:

```http
@host = http://localhost:8080

###
# @name CreateUser
POST {{host}}/post HTTP/1.1
Content-Type: application/json

{
  "name": "Ada"
}
```

## 6. Send just one request from a file with several

```sh
httpfly run -name CreateUser example.http
```

## Where to next

- [Usage](usage.md) — every command and flag
- [.http File Format](http-file-format.md) — the full request syntax:
  metadata, headers, variables, proxying
- [Environments](environments.md) — running the same file against
  dev/staging/prod without editing it
