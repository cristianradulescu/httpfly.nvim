# httpfly

A CLI HTTP client that runs `.http` files (REST Client / IntelliJ HTTP file
syntax): plain-text request definitions with variable interpolation and
pre-/post-request scripting.

## Status

Early scaffold — the CLI wiring exists but the parser is not implemented yet.

## Usage

```sh
go run ./cmd/httpfly run doc/examples/1_basic.http
```

## Development

`make httpbin-up` starts a local httpbin container so example requests don't
hit the network; see the Makefile.
