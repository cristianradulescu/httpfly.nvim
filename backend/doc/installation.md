# Installation

httpfly is a single Go binary. There are no released binaries yet, so build
it from source.

## Requirements

- Go 1.26 or later
- (Optional) Docker, if you want to try the bundled examples against a
  local server instead of the internet — see [Getting Started](getting-started.md)

## Build from source

```sh
git clone https://github.com/cristianradulescu/httpfly.git
cd httpfly
make build
```

This produces `bin/httpfly`. Run it directly:

```sh
./bin/httpfly help
```

Or put it on your `PATH`:

```sh
cp bin/httpfly /usr/local/bin/httpfly
```

## Without building a binary

For trying things out or hacking on httpfly itself, `go run` works without a
separate build step:

```sh
go run ./cmd/httpfly help
```

Every example in these docs uses `httpfly` as if it were installed; replace
it with `go run ./cmd/httpfly` if you're running from a source checkout
without installing.

## Verify it works

```sh
httpfly help
```

should print the command usage. Next: [Getting Started](getting-started.md).
