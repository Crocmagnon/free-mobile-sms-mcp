# free-mobile-sms-mcp Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** An stdio MCP server exposing one tool, `send_sms`, that texts the user driving the agent through the Free Mobile SMS notification API.

**Architecture:** Light hexagonal: `domain/sms` holds the use case and declares the `Sender` port; `adapters/freemobile` implements it over HTTP; `adapters/credentials` stores the login and key in a 0600 file; `adapters/mcp` exposes the use case as a tool; `cmd/free-mobile-sms-mcp` wires it all and adds `login`/`logout`/`version` subcommands.

**Tech Stack:** Go 1.26+, `github.com/modelcontextprotocol/go-sdk` v1.8.0, `golang.org/x/term` v0.46.0, mise, golangci-lint v2, prek, GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-10-05-free-mobile-sms-mcp-design.md` — read it before your task.

## Global Constraints

- Module path: `go.augendre.info/free-mobile-sms-mcp`. `go.mod` directive: `go 1.26.0` (CI tests `stable` and `oldstable`; x/term needs 1.26).
- Public repo: no `stash.ovh.net` import, no internal library, no internal tooling (no vendir, no devtools tasks).
- Only dependencies: go-sdk, x/term (and their transitive deps). Tests use the standard library only (no gotest.tools, no testify).
- Commits on `master`, author `Gabriel Augendre <gabriel@augendre.info>` (already set in the repo's local git config, with `commit.gpgsign false`). Do not touch git config. End every commit message with a blank line then `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Do not push. There is no remote yet.
- The API key never appears in output, errors or logs. No env var ever carries the key; `FREE_MOBILE_SMS_MCP_CREDENTIALS` names the file path only.
- Endpoint: `https://smsapi.free-mobile.fr/sendmsg`, POST, form fields `user`, `pass`, `msg`.
- Messages are forwarded untouched: no prefix, no truncation. Only empty/whitespace-only is rejected.
- No retries. HTTP timeout 30 s.
- Comments: only what the code cannot say; Go doc comments on exported identifiers say what the caller needs to know.
- File names kebab-case; JSON keys camelCase (`user`, `pass`).
- Before each commit: `mise run lint` and `mise run test` green (the pre-commit hook runs them too if installed via `prek install`; do not bypass it with `--no-verify` unless you just ran both green on the exact tree).

## Review Focus

1. Free Mobile answers a status outside the documented set (e.g. 404, 502) → an error naming the status, never success.
2. The credentials file exists but is malformed JSON or has an empty field → a clear error at `serve` startup, not on the first tool call. (Test in Task 3.)
3. A message with accents, emoji, `&`, `=`, `+`, newlines → reaches Free byte-for-byte via form encoding. (Test in Task 2.)
4. `login` run by an agent (stdin not a TTY) → refused before any prompt or SMS. (Covered by Task 5's `errNotATerminal` path.)
5. Context cancelled or network down → error wrapped with what was being done, no key in it. (Test in Task 2: error string never contains the key.)

---

## File Structure

```
go.mod, go.sum, LICENSE, .gitignore, mise.toml, .golangci.yml, .pre-commit-config.yaml, renovate.json
.github/workflows/go.yml
domain/sms/service.go, domain/sms/service_test.go
adapters/freemobile/client.go, adapters/freemobile/client_test.go
adapters/credentials/credentials.go, adapters/credentials/credentials_test.go
adapters/mcp/server.go, adapters/mcp/server_test.go
cmd/free-mobile-sms-mcp/main.go, cmd/free-mobile-sms-mcp/main_test.go
README.md, CLAUDE.md
```

---

### Task 1: Scaffold, tooling, and the domain use case

**Files:**
- Create: `go.mod`, `LICENSE`, `.gitignore`, `mise.toml`, `.golangci.yml`, `.pre-commit-config.yaml`, `renovate.json`, `.github/workflows/go.yml`
- Create: `domain/sms/service.go`, `domain/sms/service_test.go`

**Interfaces:**
- Produces: package `sms` (import `go.augendre.info/free-mobile-sms-mcp/domain/sms`):
  - `type Sender interface { Send(ctx context.Context, msg string) error }`
  - `var ErrEmptyMessage = errors.New("the message is empty")`
  - `func NewService(sender Sender) *Service`
  - `func (s *Service) Send(ctx context.Context, msg string) error`

- [ ] **Step 1: Tooling files**

`go.mod`:
```
module go.augendre.info/free-mobile-sms-mcp

go 1.26.0
```

`LICENSE`: the full Unlicense text, verbatim from https://unlicense.org/UNLICENSE (starts "This is free and unencumbered software released into the public domain.").

`.gitignore`:
```
*.test
*.out
go.work
go.work.sum
.env
.idea
dist/
/free-mobile-sms-mcp
```

`mise.toml` (no `go` pin: CI's `setup-go` matrix provides Go, and a mise pin would override it):
```toml
[env]
CGO_ENABLED = 1

[tools]
golangci-lint = "2.13.2"
prek = "latest"

[tasks.test]
run = "go test -race -v ./..."

[tasks.lint]
run = "golangci-lint run --fix"

[tasks.build]
run = "go build -o dist/ ./cmd/..."
```

`.golangci.yml`:
```yaml
version: "2"
issues:
  fix: true
linters:
  default: all
  disable:
    - depguard
    - exhaustruct
    - exhaustruct_v5
    - wsl
    - noinlineerr
    - gomodguard
  settings:
    cyclop:
      max-complexity: 20
formatters:
  enable:
    - goimports
    - gofmt
    - gofumpt
    - golines
  settings:
    goimports:
      local-prefixes:
        - go.augendre.info/free-mobile-sms-mcp
```

`.pre-commit-config.yaml`:
```yaml
repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v6.0.0
    hooks:
      - id: check-yaml
      - id: end-of-file-fixer
      - id: trailing-whitespace
  - repo: local
    hooks:
      - id: golangci-lint
        name: golangci-lint
        language: system
        types_or: [go]
        pass_filenames: false
        entry: mise run lint
      - id: go-test
        name: go test ./...
        language: system
        types_or: [go]
        pass_filenames: false
        entry: mise run test
```

`renovate.json`:
```json
{
  "$schema": "https://docs.renovatebot.com/renovate-schema.json",
  "extends": [
    "config:recommended",
    "schedule:monthly"
  ],
  "enabledManagers": [
    "github-actions",
    "mise"
  ]
}
```
Note: arangolint/fatcontext enable only `github-actions` and `mise`; add `"gomod"` here since this repo has real Go dependencies.

`.github/workflows/go.yml` (copied from arangolint, branch `master`):
```yaml
name: CI

on:
  push:
    branches: [ "master" ]
  pull_request:
    branches: [ "master" ]

permissions:
  contents: read
  pull-requests: read

jobs:
  test:
    strategy:
      fail-fast: false
      matrix:
        go: [stable, oldstable]
        os: [macos-latest, windows-latest, ubuntu-latest]
    name: test
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version: ${{ matrix.go }}
      - uses: jdx/mise-action@v4
      - name: Build
        run: go build -v ./...
      - name: Test
        run: mise run test
  check:
    name: check
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version: stable
      - uses: jdx/mise-action@v4
      - name: pre-commit
        run: prek run --all-files
  coverage:
    name: coverage
    permissions:
      contents: write
    concurrency:
      group: coverage
    runs-on: ubuntu-latest
    needs: [test]
    steps:
      - name: Update coverage report
        uses: ncruces/go-coverage-report@v0
        with:
          report: true
          chart: true
          amend: true
        if: |
          github.event_name == 'push'
        continue-on-error: true
```

Run `mise trust && mise install` so golangci-lint and prek are available, then `prek install` to install the git hook.

- [ ] **Step 2: Write the failing test** — `domain/sms/service_test.go`

```go
package sms_test

import (
	"context"
	"errors"
	"testing"

	"go.augendre.info/free-mobile-sms-mcp/domain/sms"
)

type fakeSender struct {
	got   []string
	err   error
}

func (f *fakeSender) Send(_ context.Context, msg string) error {
	f.got = append(f.got, msg)
	return f.err
}

func TestSend_ForwardsTheMessageUntouched(t *testing.T) {
	t.Parallel()

	sender := &fakeSender{}
	msg := "  Build done ✅ & tests: 42/42\nnext = deploy  "

	if err := sms.NewService(sender).Send(t.Context(), msg); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if len(sender.got) != 1 || sender.got[0] != msg {
		t.Fatalf("sender got %q, want exactly [%q]", sender.got, msg)
	}
}

func TestSend_RejectsAnEmptyMessageWithoutSending(t *testing.T) {
	t.Parallel()

	for _, msg := range []string{"", "   ", "\n\t "} {
		sender := &fakeSender{}

		err := sms.NewService(sender).Send(t.Context(), msg)
		if !errors.Is(err, sms.ErrEmptyMessage) {
			t.Errorf("Send(%q) = %v, want ErrEmptyMessage", msg, err)
		}

		if len(sender.got) != 0 {
			t.Errorf("Send(%q) reached the sender", msg)
		}
	}
}

func TestSend_ReturnsTheSenderError(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")

	err := sms.NewService(&fakeSender{err: boom}).Send(t.Context(), "hi")
	if !errors.Is(err, boom) {
		t.Fatalf("Send = %v, want it to wrap %v", err, boom)
	}
}
```

- [ ] **Step 3: Run it, expect a build failure** — `go test ./domain/...` → FAIL, package `sms` has no non-test files.

- [ ] **Step 4: Implement** — `domain/sms/service.go`

```go
// Package sms sends a text message to the user driving the agent.
package sms

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrEmptyMessage says there was nothing to send.
var ErrEmptyMessage = errors.New("the message is empty")

// Sender delivers a message to the user's phone.
type Sender interface {
	Send(ctx context.Context, msg string) error
}

// Service is the send use case.
type Service struct {
	sender Sender
}

// NewService builds the use case on top of a sender.
func NewService(sender Sender) *Service {
	return &Service{sender: sender}
}

// Send forwards msg untouched, unless it is blank.
func (s *Service) Send(ctx context.Context, msg string) error {
	if strings.TrimSpace(msg) == "" {
		return ErrEmptyMessage
	}

	if err := s.sender.Send(ctx, msg); err != nil {
		return fmt.Errorf("sending the SMS: %w", err)
	}

	return nil
}
```

- [ ] **Step 5: Verify** — `mise run test` PASS, `mise run lint` clean (fix whatever the linters flag; `t.Parallel` and naming linters may want adjustments — adjust the test code, not the config, unless a linter is clearly inapplicable).

- [ ] **Step 6: Commit** — `git add -A && git commit -m "Scaffold project and add the send use case"` (with the Co-Authored-By trailer).

---

### Task 2: Free Mobile HTTP adapter

**Files:**
- Create: `adapters/freemobile/client.go`, `adapters/freemobile/client_test.go`

**Interfaces:**
- Consumes: nothing (it satisfies `sms.Sender` structurally; do not import `domain/sms`).
- Produces: package `freemobile`:
  - `const Endpoint = "https://smsapi.free-mobile.fr/sendmsg"`
  - `var ErrBadRequest, ErrRateLimited, ErrForbidden, ErrServer error`
  - `func New(httpClient *http.Client, endpoint, user, pass string) *Client`
  - `func (c *Client) Send(ctx context.Context, msg string) error`
  - `func NewHTTPClient() *http.Client` — 30 s timeout.

Error messages (exact):
- `ErrBadRequest`: `"Free Mobile rejected the request: a required parameter is missing"`
- `ErrRateLimited`: `"Free Mobile is rate limiting: too many SMS sent in too little time, wait before retrying"`
- `ErrForbidden`: `"Free Mobile refused the credentials: the SMS notification service is not enabled in the subscriber area, or the login or key is wrong"`
- `ErrServer`: `"Free Mobile server error: try again later"`
- Any other non-200: `fmt.Errorf("unexpected answer from Free Mobile: HTTP %d", code)` (define `ErrUnexpectedStatus` and wrap it with `%w` so lint's err113 is happy).

- [ ] **Step 1: Write the failing tests** — `adapters/freemobile/client_test.go`

```go
package freemobile_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.augendre.info/free-mobile-sms-mcp/adapters/freemobile"
)

const (
	testUser = "12345678"
	testPass = "s3cr3t-k3y"
)

func TestSend_PostsTheFormFields(t *testing.T) {
	t.Parallel()

	msg := "Déploiement OK ✅ & a=b+c\nligne 2"

	var gotMethod, gotUser, gotPass, gotMsg, gotQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotQuery = r.Method, r.URL.RawQuery
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm: %v", err)
		}
		gotUser, gotPass, gotMsg = r.PostForm.Get("user"), r.PostForm.Get("pass"), r.PostForm.Get("msg")
	}))
	t.Cleanup(server.Close)

	err := freemobile.New(server.Client(), server.URL, testUser, testPass).Send(t.Context(), msg)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if gotQuery != "" {
		t.Errorf("query = %q, want nothing in the URL", gotQuery)
	}
	if gotUser != testUser || gotPass != testPass || gotMsg != msg {
		t.Errorf("form = user %q pass %q msg %q", gotUser, gotPass, gotMsg)
	}
}

func TestSend_MapsTheStatusCodes(t *testing.T) {
	t.Parallel()

	for code, want := range map[int]error{
		http.StatusBadRequest:          freemobile.ErrBadRequest,
		http.StatusPaymentRequired:     freemobile.ErrRateLimited,
		http.StatusForbidden:           freemobile.ErrForbidden,
		http.StatusInternalServerError: freemobile.ErrServer,
		http.StatusNotFound:            freemobile.ErrUnexpectedStatus,
		http.StatusBadGateway:          freemobile.ErrUnexpectedStatus,
		http.StatusCreated:             freemobile.ErrUnexpectedStatus,
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(code)
		}))

		err := freemobile.New(server.Client(), server.URL, testUser, testPass).Send(t.Context(), "hi")
		server.Close()

		if !errors.Is(err, want) {
			t.Errorf("HTTP %d: Send = %v, want %v", code, err, want)
		}
		if err != nil && strings.Contains(err.Error(), testPass) {
			t.Errorf("HTTP %d: the error leaks the key: %v", code, err)
		}
	}
}

func TestSend_UnreachableServerDoesNotLeakTheKey(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.NotFoundHandler())
	url := server.URL
	server.Close()

	err := freemobile.New(http.DefaultClient, url, testUser, testPass).Send(t.Context(), "hi")
	if err == nil {
		t.Fatal("Send succeeded against a closed server")
	}
	if strings.Contains(err.Error(), testPass) {
		t.Errorf("the error leaks the key: %v", err)
	}
}
```

The unexpected-status error must also name the code: add an assertion that the 404 error string contains `"404"`.

- [ ] **Step 2: Run** — `go test ./adapters/freemobile/` → FAIL (undefined).

- [ ] **Step 3: Implement** — `adapters/freemobile/client.go`

```go
// Package freemobile sends an SMS through the Free Mobile notification API, which only
// delivers to the line owning the credentials.
package freemobile

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Endpoint is the production API.
const Endpoint = "https://smsapi.free-mobile.fr/sendmsg"

const timeout = 30 * time.Second

var (
	// ErrBadRequest is HTTP 400.
	ErrBadRequest = errors.New("Free Mobile rejected the request: a required parameter is missing")
	// ErrRateLimited is HTTP 402.
	ErrRateLimited = errors.New("Free Mobile is rate limiting: too many SMS sent in too little time, wait before retrying")
	// ErrForbidden is HTTP 403.
	ErrForbidden = errors.New("Free Mobile refused the credentials: the SMS notification service is not enabled in the subscriber area, or the login or key is wrong")
	// ErrServer is HTTP 500.
	ErrServer = errors.New("Free Mobile server error: try again later")
	// ErrUnexpectedStatus is any other status but 200.
	ErrUnexpectedStatus = errors.New("unexpected answer from Free Mobile")
)

// Client sends SMS with one account's credentials.
type Client struct {
	httpClient *http.Client
	endpoint   string
	user       string
	pass       string
}

// NewHTTPClient is the HTTP client to give New outside tests.
func NewHTTPClient() *http.Client {
	return &http.Client{Timeout: timeout}
}

// New builds a client posting to endpoint, normally Endpoint.
func New(httpClient *http.Client, endpoint, user, pass string) *Client {
	return &Client{httpClient: httpClient, endpoint: endpoint, user: user, pass: pass}
}

// Send posts msg as a form, so the key stays out of the URL and of any error quoting it.
func (c *Client) Send(ctx context.Context, msg string) error {
	form := url.Values{"user": {c.user}, "pass": {c.pass}, "msg": {msg}}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("building the request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling Free Mobile: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusBadRequest:
		return ErrBadRequest
	case http.StatusPaymentRequired:
		return ErrRateLimited
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusInternalServerError:
		return ErrServer
	default:
		return fmt.Errorf("%w: HTTP %d", ErrUnexpectedStatus, resp.StatusCode)
	}
}
```

Lint will flag capitalised error strings (staticcheck ST1005) because they start with "Free": suppress with a `//nolint:staticcheck // the brand name starts the sentence` on the `var` block, or reword — your call, keep the meaning. Handle the unchecked `resp.Body.Close()` error the way errcheck accepts (e.g. `defer func() { _ = resp.Body.Close() }()`).

- [ ] **Step 4: Verify** — `mise run test` and `mise run lint` green.

- [ ] **Step 5: Commit** — `Add the Free Mobile HTTP client`.

---

### Task 3: Credentials file

**Files:**
- Create: `adapters/credentials/credentials.go`, `adapters/credentials/credentials_test.go`

**Interfaces:**
- Produces: package `credentials`:
  - `type Credentials struct { User string \`json:"user"\`; Pass string \`json:"pass"\` }`
  - `const PathEnv = "FREE_MOBILE_SMS_MCP_CREDENTIALS"`
  - `var ErrNoCredentials` = `"no Free Mobile credentials stored: run ` + "`free-mobile-sms-mcp login`" + ` in your own terminal"`
  - `var ErrIncomplete` = `"the credentials file needs both a user and a pass"`
  - `var ErrNoPath` = `"no user configuration directory on this platform: set FREE_MOBILE_SMS_MCP_CREDENTIALS to the file the credentials belong in"`
  - `type Store struct{ path string }`
  - `func New() Store` — `$FREE_MOBILE_SMS_MCP_CREDENTIALS` if non-empty, else `filepath.Join(os.UserConfigDir(), "free-mobile-sms-mcp", "credentials.json")`; on `UserConfigDir` error, `Store{}` (empty path, reported by the first call as `ErrNoPath`).
  - `func NewAt(path string) Store`
  - `func (s Store) Path() string`
  - `func (s Store) Get() (Credentials, error)` — absent file → `ErrNoCredentials`; bad JSON → error wrapping the decode error and naming the path; empty `User` or `Pass` → `ErrIncomplete` (wrapped with the path).
  - `func (s Store) Save(c Credentials) error` — `ErrIncomplete` if a field is empty; `os.MkdirAll(dir, 0o700)`; write the JSON to a temp file in the same dir created with mode `0o600`, then `os.Rename` onto the path (atomic, never a moment with looser mode). After rename, `os.Chmod(path, 0o600)` is unnecessary since the temp file is already 0600.
  - `func (s Store) Delete() error` — absent file is not an error.

- [ ] **Step 1: Write the failing tests** — `adapters/credentials/credentials_test.go`, each using `credentials.NewAt(filepath.Join(t.TempDir(), "sub", "credentials.json"))`:
  - `TestSaveThenGet_RoundTrips`: Save `{User:"12345678", Pass:"k3y"}`, Get returns it equal.
  - `TestSave_KeepsTheFileOwnerOnly`: after Save, `os.Stat(path).Mode().Perm() == 0o600`; skip on `runtime.GOOS == "windows"` (`t.Skip("no unix modes")`).
  - `TestGet_AbsentFile`: `errors.Is(err, credentials.ErrNoCredentials)`.
  - `TestGet_MalformedFile`: write `{not json` with 0600; Get returns non-nil error that is not `ErrNoCredentials` and contains the path.
  - `TestGet_IncompleteFile`: write `{"user":"12345678"}`; `errors.Is(err, credentials.ErrIncomplete)`.
  - `TestSave_RefusesIncomplete`: Save `{User:"x"}` → `ErrIncomplete`, and the file does not exist afterwards.
  - `TestDelete_RemovesAndToleratesAbsence`: Save, Delete → nil, Get → `ErrNoCredentials`, Delete again → nil.
  - `TestNew_HonoursTheEnv`: `t.Setenv(credentials.PathEnv, p)`; `credentials.New().Path() == p` (not parallel — `t.Setenv` forbids it).
  - `TestEmptyPath_ReportsErrNoPath`: `credentials.NewAt("").Get()` → `ErrNoPath`; same for `Save` and `Delete`.

- [ ] **Step 2: Run** — FAIL (undefined).

- [ ] **Step 3: Implement** `adapters/credentials/credentials.go` to the interface above. Package doc: `// Package credentials stores the Free Mobile login and API key in a file only the user's own account can read.` Use `json.MarshalIndent(c, "", "  ")` plus a trailing newline so the file is hand-editable. Use `os.CreateTemp(dir, ".credentials-*.json")` (CreateTemp already uses mode 0600), write, close, rename; remove the temp file on any error path.

- [ ] **Step 4: Verify** — `mise run test`, `mise run lint` green.

- [ ] **Step 5: Commit** — `Add the credentials store`.

---

### Task 4: MCP adapter

**Files:**
- Create: `adapters/mcp/server.go`, `adapters/mcp/server_test.go`
- Modify: `go.mod`, `go.sum` (`go get github.com/modelcontextprotocol/go-sdk@v1.8.0`)

**Interfaces:**
- Consumes: nothing from other packages at compile time (declares its own port).
- Produces: package `mcp` (import alias `mcpadapter` in `cmd`):
  - `type SMS interface { Send(ctx context.Context, msg string) error }` — satisfied by `*sms.Service`.
  - `func NewServer(sms SMS, version string) *Server`
  - `func (s *Server) Run(ctx context.Context) error` — stdio.
  - `func (s *Server) Serve(ctx context.Context, transport sdk.Transport) (*sdk.ServerSession, error)` — for tests.

Tool: name `send_sms`, input struct:
```go
type sendSMSArgs struct {
	Message string `json:"message" jsonschema:"the text to send, forwarded untouched"`
}
```
Description (exact): `"Send an SMS to the user running this agent, on their own phone, through the Free Mobile notification API: there is no recipient to choose, the account owning the credentials is the only one it can reach. Use it to notify them, e.g. when a long task finishes or you need their input. The message is sent untouched. A rate-limit error means too many SMS in too little time: wait before retrying."`

Handler: on success return `&sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: "SMS sent."}}}, nil, nil`; on error return `&sdk.CallToolResult{IsError: true, Content: []sdk.Content{&sdk.TextContent{Text: err.Error()}}}, nil, nil` (a tool error the agent can read, not a protocol error). Implementation name in `sdk.Implementation`: `"free-mobile-sms-mcp"`.

Reference shape (from a sibling project using the same SDK):
```go
func (s *Server) mcpServer() *sdk.Server {
	server := sdk.NewServer(&sdk.Implementation{Name: "free-mobile-sms-mcp", Version: s.version}, nil)
	sdk.AddTool(server, &sdk.Tool{Name: "send_sms", Description: sendSMSDescription}, s.sendSMS)
	return server
}

func (s *Server) Run(ctx context.Context) error {
	if err := s.mcpServer().Run(ctx, &sdk.StdioTransport{}); err != nil {
		return fmt.Errorf("serving MCP on stdio: %w", err)
	}
	return nil
}

func (s *Server) Serve(ctx context.Context, transport sdk.Transport) (*sdk.ServerSession, error) {
	session, err := s.mcpServer().Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connecting the MCP server: %w", err)
	}
	return session, nil
}

func (s *Server) sendSMS(ctx context.Context, _ *sdk.CallToolRequest, args sendSMSArgs) (*sdk.CallToolResult, any, error) { ... }
```

- [ ] **Step 1: Write the failing tests** — `adapters/mcp/server_test.go`, with this helper:

```go
func connect(t *testing.T, service mcpadapter.SMS) *sdk.ClientSession {
	t.Helper()

	clientTransport, serverTransport := sdk.NewInMemoryTransports()
	serverSession, err := mcpadapter.NewServer(service, "test").Serve(t.Context(), serverTransport)
	if err != nil {
		t.Fatalf("Serve: %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Wait() })

	clientSession, err := sdk.NewClient(&sdk.Implementation{Name: "test"}, nil).Connect(t.Context(), clientTransport, nil)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })

	return clientSession
}
```
(Cleanups run LIFO: client closes first, then the server's Wait returns.)

Tests, with a `fakeSMS{got []string; err error}`:
  - `TestTools_OnlySendSMS`: `session.ListTools` returns exactly one tool named `send_sms`, whose input schema has a required `message` property.
  - `TestSendSMS_Success`: `session.CallTool(ctx, &sdk.CallToolParams{Name: "send_sms", Arguments: map[string]any{"message": "done ✅"}})` → `IsError` false, text `"SMS sent."`, fake got `["done ✅"]`.
  - `TestSendSMS_ErrorIsAToolError`: fake returns `errors.New("rate limited")` → no protocol error, `IsError` true, text contains `"rate limited"`.

- [ ] **Step 2: Run** — FAIL.
- [ ] **Step 3: Implement** `adapters/mcp/server.go`, package doc `// Package mcp exposes the SMS use case as an MCP tool over stdio.`
- [ ] **Step 4: Verify** — `go mod tidy`, `mise run test`, `mise run lint` green.
- [ ] **Step 5: Commit** — `Add the send_sms MCP tool`.

---

### Task 5: Command, subcommands, and docs

**Files:**
- Create: `cmd/free-mobile-sms-mcp/main.go`, `cmd/free-mobile-sms-mcp/main_test.go`, `README.md`, `CLAUDE.md`
- Modify: `mise.toml` (add `mcp:dev`, `mcp:release`), `go.mod`/`go.sum` (`golang.org/x/term@v0.46.0`)

**Interfaces:**
- Consumes: `credentials.New() Store`, `Store.Get/Save/Delete/Path`, `credentials.Credentials{User, Pass}`; `freemobile.New(httpClient, endpoint, user, pass)`, `freemobile.NewHTTPClient()`, `freemobile.Endpoint`; `sms.NewService(sender)`; `mcpadapter.NewServer(sms, version).Run(ctx)`.

Behaviour:
- `free-mobile-sms-mcp [serve]` (default when no arg), `login`, `logout`, `version`; anything else prints usage to stderr and errors `unknown command "x"`. No flags. `-h`/`--help`/`help` print usage and exit 0.
- `main()` calls `run()`; on error prints `error: <msg>` to stderr, exit 1. `ctx` from `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)`.
- `serve`: `store.Get()` first — any error returned as-is (fail fast before serving). Then wire `freemobile.New(freemobile.NewHTTPClient(), freemobile.Endpoint, c.User, c.Pass)` → `sms.NewService` → `mcpadapter.NewServer(service, version()).Run(ctx)`.
- `login`: if `!term.IsTerminal(int(os.Stdin.Fd()))` return `errNotATerminal` = `"stdin is not a terminal: run ` + "`free-mobile-sms-mcp login`" + ` yourself, in your own terminal"` — before printing anything. Print to stderr: `Your login and API key are on https://mobile.free.fr/account/mes-options/notifications-sms (enable "Notifications par SMS" first).` Prompt `Free Mobile login: ` (read a line from stdin, trimmed), then `API key (hidden): ` with `term.ReadPassword`, print a newline after. Empty either → `errNothingEntered`. Send `free-mobile-sms-mcp: login OK` through the real client; on error return `fmt.Errorf("checking the credentials with a test SMS: %w", err)` and write nothing. On success `store.Save`, then print `Saved to <path>. You should have received a test SMS.`
  - To keep the reading testable, split it: `func readCredentials(in io.Reader, out io.Writer, readSecret func() ([]byte, error)) (credentials.Credentials, error)` — main passes `os.Stdin`, `os.Stderr`, and `func() ([]byte, error) { return term.ReadPassword(int(os.Stdin.Fd())) }`.
- `logout`: `store.Delete()`; print `Removed <path>.`
- `version`: as below.

```go
const devVersion = "(devel)"

func version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown build"
	}
	switch info.Main.Version {
	case devVersion:
		return "dev (working tree)"
	case "":
		return "unknown build"
	default:
		return info.Main.Version
	}
}
```

- [ ] **Step 1: Write the failing tests** — `cmd/free-mobile-sms-mcp/main_test.go` (package `main`):
  - `TestReadCredentials_ReadsLoginThenSecret`: in = `strings.NewReader("12345678\n")`, readSecret returns `[]byte("k3y")` → `{User:"12345678", Pass:"k3y"}`; out contains `"Free Mobile login: "` and does not contain `"k3y"`.
  - `TestReadCredentials_TrimsTheLogin`: in `"  12345678 \r\n"` → User `"12345678"`; secret `" k3y \n"` → Pass `"k3y"`.
  - `TestReadCredentials_RefusesEmpty`: empty login → `errNothingEntered`, readSecret not called; empty secret → `errNothingEntered`.
  - `TestVersion_IsNeverEmpty`: `version() != ""`.

- [ ] **Step 2: Run** — FAIL.
- [ ] **Step 3: Implement** `main.go` with a package doc: `// Command free-mobile-sms-mcp is an MCP server that lets a coding agent text the user driving it, plus the subcommands that user runs to store the Free Mobile credentials it needs.`
- [ ] **Step 4: mise tasks** — append to `mise.toml`:

```toml
[tasks."mcp:dev"]
description = "Point this project's agent at the working tree instead of master"
run = """
claude mcp remove free-mobile-sms -s local >/dev/null 2>&1 || true
claude mcp add --scope local free-mobile-sms -- go run {{config_root}}/cmd/free-mobile-sms-mcp
echo 'dev mode: restart the agent'
"""

[tasks."mcp:release"]
description = "Go back to the master build of the server"
run = """
claude mcp remove free-mobile-sms -s local >/dev/null 2>&1 || echo 'already in release mode'
echo 'release mode: restart the agent to pick up the master build'
"""
```

- [ ] **Step 5: README.md** — short: one-paragraph what/why (texts the user driving the agent; Free Mobile only delivers to the line owning the key); Setup: 1. enable "Notifications par SMS" in the Free Mobile subscriber area, 2. `go run go.augendre.info/free-mobile-sms-mcp/cmd/free-mobile-sms-mcp@master login` in your own terminal (sends a test SMS, writes `~/.config/free-mobile-sms-mcp/credentials.json` mode 0600; `FREE_MOBILE_SMS_MCP_CREDENTIALS` overrides the path), 3. `claude mcp add --scope user free-mobile-sms -- go run go.augendre.info/free-mobile-sms-mcp/cmd/free-mobile-sms-mcp@master`, 4. restart the agent. `logout`/`version` one-liners. Tool table: `send_sms`. Errors table (400/402/403/500). Development: `mise install`, `prek install`, `mise run test|lint|build`, `mise run mcp:dev|mcp:release`. Licence: Unlicense.
- [ ] **Step 6: CLAUDE.md** — what it is (one paragraph), commands (`mise run test|lint|build|mcp:dev|mcp:release`, one test: `go test ./adapters/freemobile -run TestSend_MapsTheStatusCodes`), architecture (the 5 packages, domain declares the port, adapters never imported by domain), constraints: the agent never sees the key (file only, path override only, never an env carrying the secret), message untouched, no retries, public repo (no internal deps), POST only (key out of the URL). Plan location `docs/plans/`.
- [ ] **Step 7: Verify** — `go mod tidy`, `mise run test`, `mise run lint`, `mise run build`, then `prek run --all-files` green. Smoke: `FREE_MOBILE_SMS_MCP_CREDENTIALS=$(mktemp -u) go run ./cmd/free-mobile-sms-mcp` fails fast with the `login` hint; `go run ./cmd/free-mobile-sms-mcp login </dev/null` fails with `errNotATerminal`; `go run ./cmd/free-mobile-sms-mcp version` prints `dev (working tree)` or a pseudo-version.
- [ ] **Step 8: Commit** — `Add the command, login flow and docs`.

---

## Progress

(Update as tasks complete; note deviations.)

- Deviation from spec: `mise.toml` does not pin `go` — CI's setup-go matrix (stable/oldstable) provides it, as in arangolint/fatcontext.
- Deviation from spec: `renovate.json` adds `gomod` to the enabled managers.
