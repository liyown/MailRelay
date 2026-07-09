# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

MailRelay is a single-machine Go service that turns authenticated email into a configuration-driven command protocol:

```
Email -> Parser -> Authentication -> Router -> Handler -> Result -> Email reply
```

SQLite is the single source of durable state (dedup keys, execution audits, generated command catalog, runtime health, queued jobs, reply outbox). There is no external database or message broker.

## Commands

Go service (run from repo root):

```bash
go build ./cmd/mailrelay      # build the binary
go test ./...                 # run all tests
go test -race ./...           # race detector (run before finishing concurrency-touching work)
go test ./internal/router/    # single package
go test -run TestName ./internal/handler/   # single test
go vet ./...
```

Web console (run from `console/`, uses pnpm):

```bash
pnpm dev            # vite dev server on 127.0.0.1
pnpm test           # vitest
pnpm typecheck      # tsc --noEmit
pnpm build:embed    # build AND copy dist/ into internal/web/ui/ for Go embedding
```

`pnpm build:embed` is the critical step: `console/scripts/embed.mjs` copies `console/dist/` into `internal/web/ui/`, which `internal/web/assets.go` embeds via `//go:embed ui`. Editing console source alone does not change the shipped binary — you must re-run `build:embed` and rebuild the Go binary.

## Architecture

The runtime is assembled by `app.Build` (`internal/app/runtime.go`), which wires config → store → router → handlers → App, and holds everything behind an `sync.RWMutex` so hot-reload can atomically swap the router and security settings without downtime.

Request flow (`App.Process` in `internal/app/app.go`):
1. `mail.Parse` extracts sender, token, command name, and params (header or JSON body).
2. `security.Authenticate` requires both an allowlisted sender and a constant-time token match.
3. `store.ClaimMessage` deduplicates on `Message-ID` (or a content hash) — a claimed message never re-executes.
4. `router.Execute` resolves the configured handler by name and runs it under a timeout.
5. Result is rendered to an email reply and written to the durable reply outbox, then delivered. **Handler execution and SMTP delivery are separated by the outbox so an SMTP retry never re-runs a handler.**

Package map (`internal/`):
- `app` — runtime assembly, hot reload, the Process/Once/Run loops, reply delivery.
- `config` — YAML load + strict validation (unknown fields, duplicate commands, bad param types, unallowlisted hosts all fail startup). `${VAR}` env references; unresolved = error.
- `command` — core `Command`, `Request`, `Result`, `Handler` interface, and `command.Error` (carries a `Kind` used for safe error classification).
- `router` — resolves handler by name, applies timeout, generates `help` discovery output. Does not know handler internals.
- `handler` — one file per handler type; all satisfy `command.Handler`. `registry.go` maps names to instances. Builtins registered in `buildRouter`: `http`, `webhook`, `workflow`, `queue` (stable) and `plugin`, `shell`, `agent`, `mcp` (experimental).
- `catalog` — computes a canonical hash of the command set; diffs on reload drive change notifications.
- `store` — all SQLite access (dedup, executions, queue leasing, reply outbox, runtime events/health). Uses `modernc.org/sqlite` (pure Go, no cgo).
- `mailbox` — IMAP receive (IDLE with polling fallback) and SMTP send; `BuildReply` formats replies.
- `security` — `Authenticate` (allowlist + constant-time token) and `NetworkPolicy` (outbound SSRF guard: rejects private/loopback/link-local/metadata addresses at validate and dial time; forbids URL templates and cross-host redirects).
- `web` — embedded console HTTP server, session auth (Argon2id). `assets.go` serves the embedded SPA. Read endpoints deliberately never return credentials, tokens, handler secrets, full mail bodies, or raw provider errors. The console can also **edit** the command catalog plus `security.http_hosts` and `runtime.catalog_notify` via `GET`/`PUT /api/v1/config/draft` (session + CSRF). Writes go through `Runtime.ApplyDraft`: it validates against the full config (`config.Load` + `Validate`) before persisting, so email can still only trigger validated commands. Persistence is a surgical `yaml.Node` edit (`config.RenderDraft`) of only those three sections — it never re-serializes the loaded `Config` (env `${VAR}` is resolved before parse, so that would bake secrets into the file); mail credentials, tokens, the session secret, `${VAR}` refs, and comments are preserved, then the temp file is atomically renamed and hot-applied. The draft endpoints (behind auth+CSRF) do expose command `config` unresolved to the admin, so keep secrets in `${VAR}`/`sensitive` params, not inlined. Invalid drafts return `422 invalid_config` with the (safe, self-authored) validation message and never touch the file.
- `cli` — the `mailrelay` subcommands (`init`, `run`, `once`, `status`, `doctor`, `replay`, `soak`, `hash-password`, `version`).

## Handler contract

Every handler implements `command.Handler` and is resolved only by its configured name — email can never select an arbitrary executable, host, or tool. When adding a handler: register it in `buildRouter` (`internal/app/runtime.go`), keep outbound HTTP going through `security.NetworkPolicy`, and return a `command.Error` with an appropriate `Kind` so `App.classify` maps it to a safe reply. `queue` and `workflow` params cannot be `sensitive` — delayed/step execution needs durable plaintext.

## Conventions and constraints

- Go 1.25. Keep the dependency set minimal (see `go.mod`).
- Security defaults are deliberate: dangerous capabilities are off until a command opts in; experimental handlers must not be used for the stable golden path.
- Never persist or log tokens, mailbox passwords, API keys, or full mail bodies. `sensitive: true` params are redacted from audit records via `security.Redact`.
- Config reload is parse → validate → build → atomic swap; an invalid reload keeps the last valid runtime and records a runtime event rather than crashing.
- The full design and accepted security boundaries live in `docs/superpowers/specs/2026-07-07-mailrelay-design.md`.
