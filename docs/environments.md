# Environments

An environment file lets the same `.http` file target different backends
(dev, staging, prod) without editing it.

## The file

`http-client.env.json` is a JSON object whose top-level keys are
environment names, plus one optional reserved key, `$shared`:

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

- Each environment is a flat `{ "variable": "value" }` object.
- `$shared` holds defaults for *every* environment. An environment's own
  value wins when both define the same name. `$shared` can't be selected
  as an environment itself.

## Where it's found

The plugin searches upward from the `.http` file's directory for
`http-client.env.json` and uses the first one it finds. That lets one
file at a project root serve `.http` files nested any depth below it
(`v1/login.http`, `v2/login.http`, ...). A subtree that needs different
environments can keep its own `http-client.env.json`, which then applies
to everything under it.

The directory holding that file is also where these live:

- `http-client.private.env.json` (see below)
- `.httpfly/state.json`, the persisted [script variables](scripting.md#persistence)
- `.httpfly/history/` and `.httpfly/downloads/`
- the base for relative paths in `< file` bodies and `cmd.exec`

If no `http-client.env.json` exists anywhere above the file, the `.http`
file's own directory is used for all of the above.

## Keeping secrets out of the committed file

An optional `http-client.private.env.json` next to `http-client.env.json`
overlays it with values you don't want committed: credentials, personal
tokens, or a local-only environment. It has the same shape:

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

- Its values override the public file's key by key.
- It can define environments the public file doesn't have (like `local`
  above).
- It's optional. Add it to your `.gitignore`:

  ```
  http-client.private.env.json
  ```

Precedence, lowest to highest: public `$shared` < public `<env>` < private
`$shared` < private `<env>`.

## Selecting one

- `:HttpEnv` opens a picker listing every environment from both files.
- `:HttpEnv dev` selects one directly.
- `:HttpEnvVars` shows the resolved variables of the selected environment,
  including values persisted by scripts, which are marked `(session)` or
  `(overridden by session)`.

The selection is shown in the winbar (`env: dev`) and remembered per
environment file, so different projects don't interfere. Selecting no
environment is fine: the file then uses only its own variables. Selecting
a name that neither file defines makes the send fail with an error listing
the available environments.

## Precedence

Environment values sit in the middle of the variable precedence, highest
to lowest:

1. A request's own local `@key = value`
2. Persisted `client.global` values set by [scripts](scripting.md)
3. The selected environment (merged as described above)
4. The `.http` file's global (prelude) `@key = value`

An environment value can reference other variables (`"api": "{{host}}/api"`);
see [Variables referencing variables](http-file-format.md#variables-referencing-variables).

## Example

[`examples/http-client.env.json`](examples/http-client.env.json) and
[`examples/http-client.private.env.json`](examples/http-client.private.env.json)
go with [`examples/2_variables.http`](examples/2_variables.http). With
`make httpbin-up` running, open `2_variables.http` and:

- `:HttpEnv dev`, then send: `host` is `http://localhost:8080`.
- `:HttpEnv dev-alt-port`, then send: `host` is `http://localhost:9090`,
  so the send fails, which shows the environment overriding the file.
- `:HttpEnv prod`: this environment comes only from the private file.
