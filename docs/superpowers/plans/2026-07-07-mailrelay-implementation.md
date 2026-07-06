# MailRelay Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the complete single-host MailRelay runtime described in `docs/superpowers/specs/2026-07-07-mailrelay-design.md`.

**Architecture:** A Go modular monolith translates authenticated MIME messages into self-describing commands, dispatches them through a handler registry, persists state in SQLite, and replies over SMTP. Built-in handlers share policy, timeout, audit, and result contracts; IMAP, SMTP, processes, and network clients sit behind narrow interfaces for local integration testing.

**Tech Stack:** Go 1.24, `gopkg.in/yaml.v3`, `modernc.org/sqlite`, `github.com/emersion/go-imap`, Go standard library.

---

## File Structure

- `cmd/mailrelay/main.go`: process entry point and exit codes.
- `internal/cli/cli.go`: `init`, `run`, `once`, `status`, `help`, and `doctor` orchestration.
- `internal/config/config.go`: YAML model, environment expansion, defaults, and validation.
- `internal/command/model.go`: Command, Parameter, Request, Context, Result, and typed errors.
- `internal/mail/parser.go`: MIME and body parsing.
- `internal/security/auth.go`: sender/token authentication and redaction.
- `internal/security/network.go`: outbound URL and resolved-address policy.
- `internal/router/router.go`: command lookup, parameter validation, help, and handler dispatch.
- `internal/catalog/catalog.go`: canonical catalog rendering, hashing, and semantic diff.
- `internal/store/store.go`: SQLite migrations, deduplication, audit, catalog, queue, and runtime state.
- `internal/handler/registry.go`: handler registration and lookup.
- `internal/handler/http.go`: HTTP and webhook handlers.
- `internal/handler/workflow.go`: composed command execution.
- `internal/handler/process.go`: plugin and restricted shell handlers.
- `internal/handler/agent.go`: OpenAI-compatible agent handler.
- `internal/handler/mcp.go`: MCP JSON-RPC handler.
- `internal/handler/queue.go`: SQLite queue handler and worker.
- `internal/mailbox/imap.go`: IMAP polling and IDLE receiver.
- `internal/mailbox/smtp.go`: SMTP responder and catalog notifications.
- `internal/app/app.go`: runtime assembly, message pipeline, hot reload, and lifecycle.
- `internal/testutil/`: local protocol fixtures shared by integration tests.
- `README.md`, `mailrelay.example.yaml`: operator documentation and complete safe example.

## Task 1: Module, Configuration, and Core Types

**Files:** Create `go.mod`, `internal/config/config_test.go`, `internal/config/config.go`, `internal/command/model_test.go`, and `internal/command/model.go`.

- [ ] Write tests that load a minimal YAML configuration, expand `${MAILRELAY_TOKEN}`, reject unresolved secrets, duplicate/reserved command names, unknown parameter types, unsafe shell executable paths, and missing HTTP allowlist hosts. The test must assert a loaded command retains its description and parameter metadata.
- [ ] Run `go test ./internal/config ./internal/command` and confirm failure because packages are absent.
- [ ] Add the module and minimal models/load-validation implementation. `config.Load(path)` returns a typed `Config`; `command.ValidateParams` returns normalized values or a typed invalid-parameter error.
- [ ] Run the focused tests and `go test ./...`; both must pass.
- [ ] Commit with `feat: add configuration and command model`.

## Task 2: Mail Parsing and Authentication

**Files:** Create `internal/mail/parser_test.go`, `internal/mail/parser.go`, `internal/security/auth_test.go`, `internal/security/auth.go`.

- [ ] Write table tests for plain `key=value`, JSON bodies, `help deploy`, MIME sender normalization, attachment metadata, Message-ID fallback hashing, header-token precedence, allowlist denial, and constant-time token acceptance.
- [ ] Run `go test ./internal/mail ./internal/security` and confirm missing implementation failures.
- [ ] Implement `mail.Parse(io.Reader)`, normalized request data, fallback IDs, and `security.Authenticate` without logging raw messages or tokens.
- [ ] Run focused and full tests; both must pass.
- [ ] Commit with `feat: parse and authenticate command mail`.

## Task 3: SQLite Store

**Files:** Create `internal/store/store_test.go`, `internal/store/store.go`.

- [ ] Write tests proving migrations are idempotent, message claim is atomic, execution records redact sensitive values, catalog snapshots round-trip, queue claims survive reopen, retry attempts are bounded, reply state can retry without command re-execution, and runtime health values persist.
- [ ] Run `go test ./internal/store` and confirm it fails for missing store APIs.
- [ ] Implement `Open`, embedded schema migration, message/execution/catalog/runtime methods, and queue insert/claim/complete/fail transactions using SQLite WAL, foreign keys, and busy timeout.
- [ ] Run focused tests, reopen tests, and `go test ./...`.
- [ ] Commit with `feat: persist runtime state in sqlite`.

## Task 4: Router, Registry, Help, and Catalog

**Files:** Create `internal/handler/registry_test.go`, `internal/handler/registry.go`, `internal/router/router_test.go`, `internal/router/router.go`, `internal/catalog/catalog_test.go`, `internal/catalog/catalog.go`.

- [ ] Write tests showing registry lookup is handler-agnostic, unknown handlers fail validation, Router dispatches normalized parameters, `help` lists configured commands, `help deploy` renders description/parameters/example, the reserved command cannot be overridden, canonical hashes are stable across map order, and diff reports Added/Removed/Updated.
- [ ] Run the three package tests and observe missing API failures.
- [ ] Implement the consumer-side Handler interface registry, Router, help renderer, catalog canonicalization, SHA-256 hash, and semantic diff.
- [ ] Run focused and full tests.
- [ ] Commit with `feat: route commands and generate discovery catalog`.

## Task 5: Protected HTTP and Webhook

**Files:** Create `internal/security/network_test.go`, `internal/security/network.go`, `internal/handler/http_test.go`, `internal/handler/http.go`.

- [ ] Write tests rejecting HTTP, loopback/private/link-local/metadata IPs, unlisted hosts, DNS rebinding candidates, and cross-host redirects; also test fixed-authority templates, successful JSON HTTP execution, standard webhook envelope, timestamp, and HMAC-SHA256 signature.
- [ ] Run focused tests and observe failures from absent policy/handlers.
- [ ] Implement a dial-time resolving transport and redirect checker, then HTTP and webhook handlers that use only configuration-selected destinations and redact response limits.
- [ ] Run focused tests and the full suite.
- [ ] Commit with `feat: add protected http and webhook handlers`.

## Task 6: Workflow and Queue

**Files:** Create `internal/handler/workflow_test.go`, `internal/handler/workflow.go`, `internal/handler/queue_test.go`, `internal/handler/queue.go`.

- [ ] Write tests for sequential and dependency execution, input mapping, cycle/self-recursion rejection, shared deadline, maximum step count, queue idempotency, lease recovery, bounded retry/backoff, and queue/workflow recursion rejection.
- [ ] Run focused tests and confirm missing implementation failures.
- [ ] Implement workflow validation/execution and a local queue handler/worker that calls a narrow command executor interface.
- [ ] Run focused and full tests.
- [ ] Commit with `feat: add workflow and sqlite queue handlers`.

## Task 7: Plugin, Restricted Shell, and Custom Registration

**Files:** Create `internal/handler/process_test.go`, `internal/handler/process.go`, extend `internal/handler/registry_test.go`.

- [ ] Write tests using a fixture child process to prove versioned JSON stdin/stdout, output caps, timeout termination, fixed executable/working directory, per-argument substitution without shell parsing, environment allowlisting, executable permission checks, and custom build-time registration.
- [ ] Run focused tests and confirm failures.
- [ ] Implement plugin and shell handlers with `exec.CommandContext`, explicit argument arrays, bounded output, direct environment construction, and configuration-only executable selection.
- [ ] Run focused/full tests and race-test the process package.
- [ ] Commit with `feat: add controlled local handlers`.

## Task 8: Agent and MCP

**Files:** Create `internal/handler/agent_test.go`, `internal/handler/agent.go`, `internal/handler/mcp_test.go`, `internal/handler/mcp.go`.

- [ ] Write local-server tests proving fixed agent endpoint/model/system prompt, API-key redaction, bounded response, tool allowlist enforcement, MCP initialize then tools/call over stdio and streamable HTTP, argument validation, timeout, and result normalization.
- [ ] Run focused tests and confirm missing handler failures.
- [ ] Implement OpenAI-compatible chat-completions requests and minimal MCP 2025 JSON-RPC clients for configured stdio/HTTP transports, using protected network/process policies.
- [ ] Run focused and full tests.
- [ ] Commit with `feat: add agent and mcp handlers`.

## Task 9: IMAP, SMTP, and Pipeline

**Files:** Create `internal/mailbox/imap_test.go`, `internal/mailbox/imap.go`, `internal/mailbox/smtp_test.go`, `internal/mailbox/smtp.go`, `internal/app/app_test.go`, `internal/app/app.go`.

- [ ] Write adapter tests for TLS IMAP fetch/mark-seen, IDLE cancellation and polling fallback, SMTP TLS/auth/thread headers, catalog notifications, and pipeline tests proving auth occurs before routing, duplicate execution is suppressed across reopen, execution audit is written, handler panics are contained, reply failure retries without re-execution, and safe errors are returned.
- [ ] Run focused tests and confirm missing adapter/pipeline failures.
- [ ] Implement protocol adapters behind `Receiver`/`Sender` interfaces and assemble the transactionally deduplicated runtime pipeline.
- [ ] Run focused, integration, and full tests.
- [ ] Commit with `feat: connect mail transport and execution pipeline`.

## Task 10: Hot Reload and Runtime Lifecycle

**Files:** Create `internal/app/runtime_test.go`, extend `internal/app/app.go`.

- [ ] Write tests proving valid file changes atomically replace commands, invalid replacements retain the previous version, Catalog diff sends before marking notified, `run` reconnects with bounded backoff, and `once` processes bounded mail/queue batches then exits.
- [ ] Run focused tests and confirm failures.
- [ ] Implement polling file watcher, immutable runtime snapshots, catalog notification transaction, receiver reconnect loop, queue worker lifecycle, and graceful context cancellation.
- [ ] Run focused tests, full tests, and race tests.
- [ ] Commit with `feat: add resilient runtime and hot reload`.

## Task 11: CLI and Doctor

**Files:** Create `internal/cli/cli_test.go`, `internal/cli/cli.go`, `cmd/mailrelay/main.go`, `mailrelay.example.yaml`.

- [ ] Write tests for all six commands, stable exit codes, safe `init`, database-backed `status`, offline structural doctor checks, optional live IMAP/SMTP/DNS checks, and unknown command usage.
- [ ] Run `go test ./internal/cli ./cmd/mailrelay` and confirm failures.
- [ ] Implement dependency-injected CLI dispatch and a thin main. Generate a complete example with environment-backed secrets and disabled dangerous handlers.
- [ ] Run CLI tests, full tests, `go vet ./...`, and build `./cmd/mailrelay`.
- [ ] Commit with `feat: add mailrelay cli and diagnostics`.

## Task 12: Documentation and End-to-End Verification

**Files:** Create `README.md`, `internal/testutil/e2e_test.go`; update plan checkboxes.

- [ ] Write a local end-to-end test that runs init, opens SQLite, injects authenticated `help` mail through test adapters, verifies a threaded SMTP reply and catalog content, then checks status and doctor without public network access.
- [ ] Run the test and confirm it fails before the final wiring/documentation adjustments.
- [ ] Complete wiring and document installation, five-minute setup, configuration schema, every handler, security boundaries, Discovery, CLI usage, operations, backup, and troubleshooting.
- [ ] Run `gofmt -w`, `go test ./...`, `go test -race ./...`, `go vet ./...`, `go build -o ./bin/mailrelay ./cmd/mailrelay`, and CLI smoke tests for `help`, `init`, `doctor`, and `status`.
- [ ] Audit every design requirement against code/tests, remove generated binaries and temporary data, run `git diff --check`, then commit with `docs: complete MailRelay operator guide`.
