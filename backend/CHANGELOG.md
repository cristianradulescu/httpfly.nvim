# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

## [0.1.0] - 2026-08-19

### Added

- `.http` file parsing: `###`-delimited request blocks, `# @key value`
  metadata, headers, and bodies.
- `httpfly validate` — reports every issue in a file (missing/empty
  `@name`, malformed request lines and headers, relative URLs,
  non-standard methods, unrecognized protocol versions, unknown metadata,
  undefined variables, ...) at `ERROR` or `WARN` severity, instead of
  stopping at the first problem.
- `httpfly run` — sends the requests a file defines and prints their
  responses.
- `{{variable}}` interpolation in the URL, headers, and body, with
  `key = value` declarations that are either global (declared in the
  file's prelude, before the first `###`) or local to one request block,
  local always overriding global.
- Automatic percent-encoding of a variable's value when it lands inside a
  query parameter's value, so values containing spaces or `&` don't
  corrupt the request; the URL's own scheme/host/path structure is left
  untouched.
- `@proxy` metadata to route a request (or, declared globally, every
  request) through an HTTP proxy.
- Environments: `httpfly.env.json` alongside a `.http` file, selected with
  `-env <name>`, supplying variables that override the file's own globals
  but not a request's local ones. Supports a `shared` section merged as
  defaults across every named environment.
- `-name` to restrict `run`/`validate` to a single request.
- `-s`/`-silent` to print only response bodies, like `curl -s`.
- `-json` for structured output: one object per request with the request
  sent, the response (or transport error) received, the final URL after
  any redirects, and duration.
- `-v`/`-verbose` to also report TLS connection details (version, cipher
  suite, ALPN protocol, peer certificate chain).
- `version` command and `-version`/`--version` flags.
