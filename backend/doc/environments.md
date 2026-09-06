# Environments

An environment file lets the same `.http` file target different backends
— dev, staging, prod — without editing it.

## The file

httpfly looks for a file named `http-client.env.json` **in the current
working directory — wherever you launch `httpfly` from — not the directory
containing the `.http` file** you're running. This is deliberate: it lets
several `.http` files in different subdirectories (e.g. `v1/`, `v2/` of an
API) share one environment file in their common parent, as long as you run
httpfly from that parent:

```sh
cd my-api/            # http-client.env.json lives here
httpfly run -env dev v1/login.http
httpfly run -env dev v2/login.http   # same env file, different .http file
```

It's a JSON object with environment names as top-level keys, plus one
optional reserved key:

```json
{
  "$shared": {
    "api_version": "v2"
  },
  "dev": {
    "host": "http://localhost:8080"
  },
  "prod": {
    "host": "https://api.example.com",
    "api_key": "prod-key-here"
  }
}
```

- Every other top-level key is an environment name, each a flat
  `{ "variable": "value" }` object. This is what `-env <name>` selects
  from.
- **`$shared`** — variables applied as defaults to *every* environment,
  overridden by that environment's own value on a name conflict. Use it
  for values that don't change between environments (so you're not
  repeating them in every block), or as a fallback for something most
  environments share. It's dollar-prefixed so it can't collide with a real
  environment you happen to name `shared`.

`$shared` is not itself a selectable environment — `-env $shared` is an
error.

## Keeping secrets out of the committed file

An optional sibling file, `http-client.private.env.json`, in the same
directory, overlays the public file — for values you don't want committed
(credentials, personal tokens, a fully local-only environment). It's the
same shape (top-level environment names, optional `$shared`):

```json
{
  "$shared": {
    "api_key": "my-personal-key"
  },
  "local": {
    "host": "http://localhost:1234"
  }
}
```

- Its values override the public file's on a per-key basis, for any
  environment both files define.
- It can also define an environment the public file doesn't have at all
  (like `local` above) — that's valid, not rejected.
- A missing private file is normal, not an error — it's entirely optional.
- httpfly doesn't add it to `.gitignore` for you; do that yourself so it's
  never committed alongside `http-client.env.json`.

Precedence, lowest to highest: public `$shared` < public `<env>` < private
`$shared` < private `<env>` — so a private-file value always wins over a
public-file one, and each file's own environment entry still wins over
that same file's `$shared` defaults.

## Selecting one

```sh
httpfly run -env prod api.http
httpfly validate -env dev api.http
```

If `http-client.env.json` doesn't exist in the current working directory,
or neither file defines the named environment, httpfly reports an error
rather than silently running with nothing:

```
httpfly: run: http-client.env.json: no environment named "staging" (available: dev, prod)
```

`-env` is entirely optional — a file with no `http-client.env.json`, or
run without `-env`, behaves exactly as if environments didn't exist.

Note that `-name`, `-json`, `-v`, etc. don't affect where httpfly looks for
`http-client.env.json`/`http-client.private.env.json` — it's always the
current working directory, regardless of which `.http` file you point at
or where it lives.

## Precedence

Environment variables sit in the middle of httpfly's variable precedence,
highest to lowest:

1. A request's own **local** `@key = value` declaration
2. The selected **environment**'s value (see the precedence chain above,
   between the public and private files)
3. The `.http` file's **global** (prelude) `@key = value` declaration

In other words: the file's own globals are the baseline default, an
environment can override them, and any one request can still override
that for itself. See [.http File Format](http-file-format.md#variables)
for how global vs. local variables work within the file itself.

## Example

`doc/examples/http-client.env.json` and
`doc/examples/http-client.private.env.json` in this repo, alongside
`doc/examples/2_variables.http`. Since env-file lookup uses the current
working directory, run these from inside `doc/examples/`:

```sh
cd doc/examples
httpfly run -env dev 2_variables.http           # host = http://localhost:8080 (matches the file's own default)
httpfly run -env dev-alt-port 2_variables.http  # host = http://localhost:9090 (overrides it)
httpfly run -env prod 2_variables.http          # host from the private file -- "prod" isn't in the public one
```
