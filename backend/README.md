# httpfly

A CLI HTTP client that runs `.http` files (REST Client / IntelliJ HTTP file
syntax): plain-text request definitions with variables, environments,
proxy support, Lua pre-/post-request scripting, and converting to/from a
bash-style curl command.

```sh
httpfly run doc/examples/1_basic.http
```

## Documentation

- [Installation](doc/installation.md)
- [Getting Started](doc/getting-started.md)
- [Usage](doc/usage.md) — commands, flags, JSON output, exit codes
- [.http File Format](doc/http-file-format.md) — requests, metadata, variables
- [Environments](doc/environments.md) — running the same file against dev/staging/prod
- [Scripting](doc/scripting.md) — Lua pre-/post-request scripts, persisted variables
- [Changelog](CHANGELOG.md)

## Status

Parsing, validation, `{{variable}}` interpolation, sending requests,
environments, proxying, Lua pre-/post-request scripting, and converting
both ways between `.http` and a bash-style curl command all work.

## Development

`make check` runs fmt, vet, and the test suite. `make httpbin-up`/
`httpbin-down` start and stop a local httpbin container that the examples
under `doc/examples/` use, so trying them doesn't require network access.
See the Makefile for the full list of targets.
