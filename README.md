# free-mobile-sms-mcp

MCP server that lets a coding agent text you when it needs you: a long task finished, a decision is blocked, a prompt is waiting.

It uses the [Free Mobile SMS notification API](https://mobile.free.fr/account/), which only delivers to the line owning the key. The agent can text you, nobody else.

## Setup

1. In the Free Mobile subscriber area (https://mobile.free.fr/account/), under Mes Options > Notifications par SMS, enable the option. It shows your login and API key.
2. In your own terminal, run:
   ```sh
   go run go.augendre.info/free-mobile-sms-mcp/cmd/free-mobile-sms-mcp@main login
   ```
   It sends a test SMS, then writes `credentials.json` (mode 0600) in your user config directory (e.g. `~/.config/free-mobile-sms-mcp/credentials.json` on Linux); `login` prints the exact path. `FREE_MOBILE_SMS_MCP_CREDENTIALS` overrides the path. `login` refuses to run unless stdin is a terminal, so an agent cannot run it, and the agent never sees the key.
3. Register the server:
   ```sh
   claude mcp add --scope user free-mobile-sms -- go run go.augendre.info/free-mobile-sms-mcp/cmd/free-mobile-sms-mcp@main
   ```
4. Restart the agent.

`logout` deletes the credentials file. `version` prints the build version.

## Tool

| Tool | Input | Effect |
| --- | --- | --- |
| `send_sms` | `message` (string) | Texts you the message, untouched. Empty or whitespace-only messages are rejected. |

## Errors

| Status | Meaning |
| --- | --- |
| 400 | a required parameter is missing |
| 402 | too many SMS in too little time: wait before retrying |
| 403 | option not enabled, or wrong login/key |
| 500 | Free Mobile server error |
| other | unexpected answer: the error names the HTTP status |

There are no retries: the agent decides what to do.

## Development

```sh
mise install
prek install
mise run test
mise run lint
mise run build
```

`mise run mcp:dev` points this project's agent at the working tree, `mise run mcp:release` goes back to the main build.

## Licence

[Unlicense](LICENSE).
