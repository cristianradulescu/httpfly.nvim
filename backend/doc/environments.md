# Environments

An environment file lets the same `.http` file target different backends
— dev, staging, prod — without editing it.

## The file

httpfly looks for a file named `httpfly.env.json` **in the same directory
as the `.http` file** you're running. It's a JSON object with two optional
top-level keys:

```json
{
  "shared": {
    "api_version": "v2"
  },
  "environments": {
    "dev": {
      "host": "http://localhost:8080"
    },
    "prod": {
      "host": "https://api.example.com",
      "api_key": "prod-key-here"
    }
  }
}
```

- **`environments`** — one entry per environment name, each a flat
  `{ "variable": "value" }` object. This is what `-env <name>` selects
  from.
- **`shared`** — variables applied as defaults to *every* environment,
  overridden by that environment's own value on a name conflict. Use it
  for values that don't change between environments (so you're not
  repeating them in every block), or as a fallback for something most
  environments share.

`shared` is not itself a selectable environment — `-env shared` is an
error.

## Selecting one

```sh
httpfly run -env prod api.http
httpfly validate -env dev api.http
```

If `httpfly.env.json` doesn't exist next to `api.http`, or doesn't define
the named environment, httpfly reports an error rather than silently
running with nothing:

```
httpfly: run: httpfly.env.json: no environment named "staging" (available: dev, prod)
```

`-env` is entirely optional — a file with no `httpfly.env.json`, or run
without `-env`, behaves exactly as if environments didn't exist.

## Precedence

Environment variables sit in the middle of httpfly's variable precedence,
highest to lowest:

1. A request's own **local** `key = value` declaration
2. The selected **environment**'s value (`shared`, overridden by the
   environment's own entry)
3. The `.http` file's **global** (prelude) `key = value` declaration

In other words: the file's own globals are the baseline default, an
environment can override them, and any one request can still override
that for itself. See [.http File Format](http-file-format.md#variables)
for how global vs. local variables work within the file itself.

## Example

`doc/examples/httpfly.env.json` in this repo, alongside
`doc/examples/2_variables.http`:

```sh
httpfly run -env dev doc/examples/2_variables.http           # host = http://localhost:8080 (matches the file's own default)
httpfly run -env dev-alt-port doc/examples/2_variables.http  # host = http://localhost:9090 (overrides it)
```
