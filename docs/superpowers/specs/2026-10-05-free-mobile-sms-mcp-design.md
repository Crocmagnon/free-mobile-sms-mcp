# free-mobile-sms-mcp — design

## Goal

An MCP server, in Go, that lets a coding agent notify the human driving it by SMS, through
the Free Mobile "notifications par SMS" API. That API only delivers to the line owning the
API key, so the recipient is always the user: there is no recipient parameter, and no way to
text anyone else.

Single user, stdio, no listener, no database, no deploy. Public repository: no
employer-internal library or import. `main` is the release; installation is
`go run go.augendre.info/free-mobile-sms-mcp/cmd/free-mobile-sms-mcp@main`.

## Free Mobile API

`POST https://smsapi.free-mobile.fr/sendmsg`, form-encoded, fields `user` (login), `pass`
(API key), `msg` (message). The status code is the only result:

| Status | Meaning | Error surfaced |
| --- | --- | --- |
| 200 | sent | — |
| 400 | a required parameter is missing | `ErrBadRequest` |
| 402 | too many SMS in too little time | `ErrRateLimited` |
| 403 | service not enabled, or wrong login/key | `ErrForbidden` |
| 500 | server error | `ErrServer` |
| other | unexpected | error naming the status code |

POST rather than GET keeps the key out of the URL, so out of any error that embeds one.

## Layout

```
cmd/free-mobile-sms-mcp/main.go   subcommands serve (default) | login | logout | version
adapters/credentials/             the 0600 credentials file
adapters/freemobile/              HTTP client for the API above
domain/sms/                       Service.Send; declares the Sender port it needs
adapters/mcp/                     the send_sms tool over stdio
```

Module path `go.augendre.info/free-mobile-sms-mcp`. Dependencies:
`github.com/modelcontextprotocol/go-sdk` (MCP), `golang.org/x/term` (no-echo prompt). Tests use
the standard library only.

## Components

### adapters/credentials

JSON file `{"user": "...", "pass": "..."}`, mode `0600`, directory `0700`. Path:
`$FREE_MOBILE_SMS_MCP_CREDENTIALS` if set, else
`os.UserConfigDir()/free-mobile-sms-mcp/credentials.json`. `Store` has `Get`, `Save`,
`Delete`, `Path`. `Get` returns `ErrNoCredentials` (pointing at `login`) when the file is
absent, and an error when either field is empty. No env var ever carries the secret itself.

### adapters/freemobile

`Client{httpClient, endpoint, user, pass}` with `Send(ctx, msg) error`: one POST, status
mapped per the table. The endpoint is a field so tests point it at `httptest.Server`. The
HTTP client has a 30 s timeout. No retry: on 402 the agent decides.

### domain/sms

`Service.Send(ctx, msg)` rejects a message that is empty or whitespace-only
(`ErrEmptyMessage`), otherwise forwards it untouched to its `Sender` interface. No
truncation, no prefix: Free Mobile decides what it accepts.

### adapters/mcp

One tool, `send_sms`, input `{message: string}`. Its description says it texts the user
running the agent on their own phone, that the message is sent untouched, and that 402
means wait before retrying. Success returns the text `SMS sent.`; an error is returned as a
tool error carrying the message. Exposes `Run` (stdio) and `Serve` (any transport, for tests),
as similar MCP servers do.

### cmd/free-mobile-sms-mcp

- `serve`: reads the credentials at startup and fails fast if they are missing, then serves.
- `login`: refuses unless stdin is a terminal (so an agent cannot run it). Prompts for the
  login, then the key without echo, sends the test SMS
  `free-mobile-sms-mcp: login OK`, and writes the file only if that succeeded.
- `logout`: deletes the file; absent is not an error.
- `version`: `debug.ReadBuildInfo`, `dev (working tree)` for a `(devel)` build.

The key never appears in output, errors, or logs.

## Testing

- `freemobile`: `httptest.Server` asserting method, form fields, and each status mapping.
- `sms`: a fake `Sender`; empty-message rejection, pass-through.
- `mcp`: in-memory transport session calling `send_sms`, success and error.
- `credentials`: temp dir; round trip, file mode, absent file, incomplete file.

## Tooling

Uniform with `arangolint` and `fatcontext`:

- `mise.toml`: `CGO_ENABLED=1` (for `-race`), tools go, golangci-lint, prek; tasks `test`
  (`go test -race -v ./...`), `lint` (`golangci-lint run --fix`), `build`, plus `mcp:dev` and
  `mcp:release` toggling the local Claude Code registration between the working tree and
  `@main`.
- `.golangci.yml`: `default: all` minus depguard, exhaustruct, exhaustruct_v5, wsl,
  noinlineerr, gomodguard; formatters goimports, gofmt, gofumpt, golines; `issues.fix`.
- `.pre-commit-config.yaml`: pre-commit-hooks (check-yaml, end-of-file-fixer,
  trailing-whitespace) plus local `mise run lint` and `mise run test`. Run with prek.
- `.github/workflows/go.yml`: the same CI as those repos (test matrix stable/oldstable × 3
  OSes, `check` job running `prek run --all-files`, coverage report), on `main`.
- `renovate.json`: copied as-is. No goreleaser: there is nothing to release.

## Docs and licence

`README.md` (what it does, setup: register, login, restart the agent; tools), `CLAUDE.md`,
`LICENSE` (Unlicense). No CHANGELOG.

## Out of scope

Retries, message prefix or truncation, multiple accounts, an env var holding the key.
