# httpfly

A CLI HTTP client that runs `.http` files (REST Client / IntelliJ HTTP file
syntax): plain-text request definitions with variable interpolation and
pre-/post-request scripting.

## Status

Parsing, validation, `{{variable}}` interpolation, and sending requests all
work. Pre-/post-request scripting (`< {% %}` / `> {% %}`) is recognized by
the validator but not yet implemented.

## Usage

```sh
go run ./cmd/httpfly run doc/examples/1_basic.http
go run ./cmd/httpfly run -name Post doc/examples/1_basic.http
go run ./cmd/httpfly validate doc/examples/invalid.http
```

`-name` restricts either command to the single request declared with
`# @name X`. See `doc/examples/` for a walkthrough (`1_basic.http`,
`2_variables.http`) and `doc/examples/invalid.http` for a fixture covering
every validation rule.

## Development

`make httpbin-up` starts a local httpbin container so example requests don't
hit the network; see the Makefile. `make check` runs fmt, vet, and the test
suite.
