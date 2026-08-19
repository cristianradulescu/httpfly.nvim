# httpfly

A CLI HTTP client that runs `.http` files (REST Client / IntelliJ HTTP file
syntax): plain-text request definitions with variables, environments, and
proxy support.

```sh
httpfly run doc/examples/1_basic.http
```

## Documentation

- [Installation](doc/installation.md)
- [Getting Started](doc/getting-started.md)
- [Usage](doc/usage.md) — commands, flags, JSON output, exit codes
- [.http File Format](doc/http-file-format.md) — requests, metadata, variables
- [Environments](doc/environments.md) — running the same file against dev/staging/prod
- [Changelog](CHANGELOG.md)

## Status

Parsing, validation, `{{variable}}` interpolation, sending requests,
environments, and proxying all work. Pre-/post-request scripting
(`< {% %}` / `> {% %}`) is recognized by the validator but not yet
implemented.

## Development

`make check` runs fmt, vet, and the test suite. `make httpbin-up`/
`httpbin-down` start and stop a local httpbin container that the examples
under `doc/examples/` use, so trying them doesn't require network access.
See the Makefile for the full list of targets.
