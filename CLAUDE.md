# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`free-mobile-sms-mcp` is a stdio MCP server exposing one tool, `send_sms`, that texts the user driving the agent through the Free Mobile notification API. Subcommands `login`, `logout` and `version` are run by the user, not the agent. The design lives in `docs/superpowers/specs/`, phased plans in `docs/plans/`.

## Commands

```bash
mise run test          # go test -race -v ./...
mise run lint          # golangci-lint run --fix
mise run build         # go build -o dist/ ./cmd/...
mise run mcp:dev       # register the working tree as the agent's MCP server
mise run mcp:release   # back to the main build
go test ./adapters/freemobile -run TestSend_MapsTheStatusCodes   # one test
```

## Architecture

- `cmd/free-mobile-sms-mcp/` wires everything and dispatches `serve` (default), `login`, `logout`, `version`.
- `domain/sms/` is the use case (`Service.Send`) and declares the `Sender` port it needs.
- `adapters/freemobile/` implements `Sender` over HTTP.
- `adapters/credentials/` stores the login and key in a 0600 file.
- `adapters/mcp/` exposes the use case as the `send_sms` tool over stdio.

The domain never imports an adapter.

## Constraints

- The agent never sees the key: it lives in the credentials file only. The path can be overridden by env (`FREE_MOBILE_SMS_MCP_CREDENTIALS`), the secret never. Nothing prints the key or the file contents.
- The message is forwarded untouched: no prefix, no truncation. Only empty or whitespace-only is rejected.
- No retries.
- POST only, so the key stays out of the URL and out of errors.
- Public repository: no internal dependencies or tooling.
