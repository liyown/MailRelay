# MailRelay

> Turn authenticated email into safe, auditable automation commands.

A single-binary Go service that converts authenticated email into declared
commands, dispatches them through a handler registry, and replies through SMTP.
SQLite stores deduplication keys, execution audits, the generated command
catalog, runtime health, and queued jobs.

[Documentation](https://liyown.github.io/MailRelay/docs) ·
[GitHub Pages](https://liyown.github.io/MailRelay/) ·
[Repository](https://github.com/liyown/MailRelay)

## Why MailRelay

- **Configuration-driven commands.** Define a command, its parameters, and its
  handler in `mailrelay.yaml`. Email supplies only declared parameters. No SDK,
  no runtime code.
- **Maturity-tagged handlers.** Every handler ships with a stability label
  (`stable`, `beta`, `experimental`) so operators know what is safe for the
  golden path.
- **Auditable by default.** SQLite records every message, deduplication,
  execution, and SMTP reply. A durable outbox and dead-letter replay keep
  handler execution and SMTP delivery independent.

## Flow

```text
Email → Parser → Authentication → Router → Handler → Result → Email reply
```

## Five-minute setup

```bash
go install github.com/liyown/MailRelay/cmd/mailrelay@latest
mailrelay init
```

Edit `mailrelay.yaml`, replace every `change-me` value, set the sender
allowlist, and declare commands. Check the installation before running it:

```bash
mailrelay doctor
mailrelay once
mailrelay run
```

Secrets can be environment references such as `${IMAP_PASSWORD}`. An
unresolved reference is rejected. Keep the configuration file readable only
by the service account. See
[Installation](https://liyown.github.io/MailRelay/docs/getting-started/installation)
and
[Configuration](https://liyown.github.io/MailRelay/docs/getting-started/configuration)
for the full key list.

## Declare a command

```yaml
commands:
  - name: push
    description: Send a notification
    handler: http
    parameters:
      message:
        description: Notification text
        type: string
        required: true
        example: hello
    config:
      method: POST
      url: https://api.example.com/push
      body: '{"message":"{{message}}"}'
```

A header token takes precedence; the reserved `_token` body field is for
clients that cannot add headers. `Message-ID` deduplicates; messages without
one receive a deterministic content hash. See
[First command](https://liyown.github.io/MailRelay/docs/getting-started/first-command)
for the end-to-end walkthrough.

## Discovery

Discovery is generated from `commands`; there is no separate manual.

- Subject `help` returns every command and description.
- Subject `help deploy` returns the command description, parameters, required
  markers, and examples.
- Startup and valid configuration reloads calculate a canonical Catalog hash.
- SQLite stores the catalog snapshot. Changes produce `Added`, `Removed`, and
  `Updated` sections and notify `runtime.catalog_notify` recipients.

See [Discovery](https://liyown.github.io/MailRelay/docs/concepts/discovery)
for the request flow and notification format.

## Handlers

| Handler    | Stability     | Transport                       | Reference |
| ---------- | ------------- | ------------------------------- | --------- |
| `http`     | Stable        | HTTPS, allowlisted hostname     | [http-webhook](https://liyown.github.io/MailRelay/docs/handlers/http-webhook) |
| `webhook`  | Stable        | HTTPS with HMAC signature       | [http-webhook](https://liyown.github.io/MailRelay/docs/handlers/http-webhook) |
| `workflow` | Beta          | Internal command composition    | [workflow-queue](https://liyown.github.io/MailRelay/docs/handlers/workflow-queue) |
| `queue`    | Beta          | Local SQLite worker             | [workflow-queue](https://liyown.github.io/MailRelay/docs/handlers/workflow-queue) |
| `plugin`   | Experimental  | Subprocess, JSON stdio          | [plugin-shell](https://liyown.github.io/MailRelay/docs/handlers/plugin-shell) |
| `shell`    | Experimental  | Subprocess, argument vector     | [plugin-shell](https://liyown.github.io/MailRelay/docs/handlers/plugin-shell) |
| `agent`    | Experimental  | OpenAI-compatible chat API      | [agent-mcp](https://liyown.github.io/MailRelay/docs/handlers/agent-mcp) |
| `mcp`      | Experimental  | JSON-RPC over HTTP or stdio     | [agent-mcp](https://liyown.github.io/MailRelay/docs/handlers/agent-mcp) |

Custom Go handlers can be registered by applications embedding MailRelay; the
runtime does not load arbitrary code. Full configuration keys, transport
limits, and maturity criteria live in the
[handler reference](https://liyown.github.io/MailRelay/docs/handlers).

## Security and reliability

- Dangerous capabilities are disabled until a command explicitly selects them.
- Every request requires both an allowlisted sender and a constant-time token
  match.
- Outbound HTTP, agent, and MCP hosts must appear in `security.http_hosts`.
  Resolved private, loopback, link-local, multicast, unspecified,
  carrier-grade NAT, and metadata addresses are rejected at validation and
  dial time. Cross-host redirects are rejected.
- SMTP delivery and handler execution are decoupled by a durable SQLite
  outbox; SMTP retry never executes a handler twice.
- Exhausted queue jobs and replies stay in dead-letter state until an
  operator calls `mailrelay replay`.
- Logs and SQLite never store tokens, mailbox passwords, API keys, or full
  mail bodies.

Full security model:
[Security](https://liyown.github.io/MailRelay/docs/concepts/security).
Reliability and 72-hour acceptance:
[Reliability](https://liyown.github.io/MailRelay/docs/operations/reliability).

## CLI

```text
mailrelay init             create a safe single-file configuration
mailrelay run              run IMAP IDLE, polling fallback, queue worker, and hot reload
mailrelay once             process one bounded mail and queue batch
mailrelay status           show queue depth, Catalog hash, and latest execution
mailrelay doctor           validate configuration, addresses, SQLite, and command policies
mailrelay replay queue ID  replay one dead queue job
mailrelay replay reply ID  replay one dead SMTP reply
mailrelay soak --duration 72h  run the live reliability acceptance check
mailrelay version          print version, commit, and build time
mailrelay help             print CLI usage
```

Full reference:
[CLI](https://liyown.github.io/MailRelay/docs/operations/cli).

## Development

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/mailrelay
```

The architecture and accepted security boundaries are documented in
[`docs/superpowers/specs/2026-07-07-mailrelay-design.md`](docs/superpowers/specs/2026-07-07-mailrelay-design.md)
in this repository.

## Documentation

- [Installation](https://liyown.github.io/MailRelay/docs/getting-started/installation)
- [First command](https://liyown.github.io/MailRelay/docs/getting-started/first-command)
- [Configuration](https://liyown.github.io/MailRelay/docs/getting-started/configuration)
- [Architecture](https://liyown.github.io/MailRelay/docs/concepts/architecture)
- [Discovery](https://liyown.github.io/MailRelay/docs/concepts/discovery)
- [Security](https://liyown.github.io/MailRelay/docs/concepts/security)
- [Storage](https://liyown.github.io/MailRelay/docs/concepts/storage)
- [Handlers](https://liyown.github.io/MailRelay/docs/handlers)
- [CLI](https://liyown.github.io/MailRelay/docs/operations/cli)
- [Reliability and 72-hour acceptance](https://liyown.github.io/MailRelay/docs/operations/reliability)
- [GitHub Pages deployment](https://liyown.github.io/MailRelay/docs/operations/github-pages)
