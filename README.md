# MailRelay

MailRelay turns authenticated email into a small, configuration-driven command protocol:

```text
Email -> Parser -> Authentication -> Router -> Handler -> Result -> Email reply
```

It is a single-machine Go service. SQLite stores deduplication keys, execution audits, the generated command catalog, runtime health, and queued jobs.

## Five-minute setup

```bash
go install github.com/liyown/MailRelay/cmd/mailrelay@latest
mailrelay init
```

Edit `mailrelay.yaml`, replace all `change-me` values, set the sender allowlist, and declare commands. Check the installation before running it:

```bash
mailrelay doctor
mailrelay once
mailrelay run
```

Secrets can be environment references such as `${IMAP_PASSWORD}`. An unresolved reference is rejected. Keep the configuration file readable only by the service account.

## Sending a command

Send mail from an allowlisted address:

```text
Subject: push
X-MailRelay-Token: your-token

message=hello
```

JSON bodies are supported with `Content-Type: application/json`. The reserved `_token` body field can be used when the sending client cannot add headers. A header token takes precedence. `Message-ID` is used for durable deduplication; messages without one receive a deterministic content hash.

## Discovery

Discovery is generated from `commands`; there is no separate command manual.

- Subject `help` returns every command and description.
- Subject `help deploy` returns the command description, parameters, required markers, and examples.
- Startup and valid configuration reloads calculate a canonical Catalog hash.
- SQLite stores the Catalog snapshot. Changes produce `Added`, `Removed`, and `Updated` sections and notify `runtime.catalog_notify` recipients.

Example command:

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

Parameter types are `string`, `integer`, `number`, and `boolean`. Mark a parameter `sensitive: true` to redact it from audit records.

## Handlers

All handlers implement the same consumer-side interface. The Router only resolves the configured handler name.

Support levels are part of generated Discovery output:

- **Stable:** `http`, `webhook` — supported for the v0.1 golden path.
- **Beta:** `workflow`, `queue` — suitable for controlled use with audit monitoring.
- **Experimental:** `plugin`, `shell`, `agent`, `mcp`, and custom handlers — APIs and safety limits may still change.

### `http`

Calls a fixed HTTPS URL. The hostname must be listed in `security.http_hosts`. URL templates are forbidden. Resolved private, loopback, link-local, multicast, unspecified, carrier-grade NAT, and metadata addresses are rejected at validation and dial time. Cross-host redirects are rejected.

Configuration keys: `url`, `method`, `headers`, and `body`.

### `webhook`

POSTs a standard JSON envelope containing version, command, request ID, timestamp, and parameters. `secret` adds an `X-MailRelay-Signature: sha256=...` HMAC header. It uses the same outbound policy as HTTP.

### `workflow`

Runs a bounded list of configured command steps through the Router. Each step has `command` and optional `params`. Values such as `{{env}}` map request parameters. Direct recursion, missing targets, timeouts, and excessive step counts fail safely.

### `queue`

Inserts a target command into the SQLite queue with an idempotency key derived from the message. `max_attempts` bounds retry. The local worker leases jobs transactionally and recovers expired leases after restart.

### `plugin`

Starts a configured absolute executable without a shell. It writes a versioned JSON request to stdin and expects a JSON `Result` on stdout. Output and environment are bounded.

### `shell`

Starts a configured absolute executable directly with an argument array. Each argument is expanded independently, so characters such as `;`, `$()`, and pipes are data rather than shell syntax. Arbitrary executables cannot be selected by email.

### `agent`

Calls a configured OpenAI-compatible chat-completions endpoint. Endpoint, model, system prompt, and API key are fixed by configuration. Email supplies only declared parameters. Add the endpoint host to `security.http_hosts`.

### `mcp`

Calls one allowlisted MCP tool using JSON-RPC over configured HTTP or stdio transport. HTTP is subject to outbound network policy; stdio uses a fixed executable. Email cannot select a server or unlisted tool.

### Custom Go handlers

Applications embedding MailRelay may pass additional `command.Handler` implementations to `app.Build`. Runtime loading of arbitrary code is intentionally unsupported.

## CLI

```text
mailrelay init     create a safe single-file configuration
mailrelay run      run IMAP IDLE, polling fallback, queue worker, and hot reload
mailrelay once     process one bounded mail and queue batch
mailrelay status   show queue depth, Catalog hash, and latest execution
mailrelay doctor   validate configuration, addresses, SQLite, and command policies
mailrelay replay queue ID  replay one dead queue job
mailrelay replay reply ID  replay one dead SMTP reply
mailrelay soak --duration 72h  run the live reliability acceptance check
mailrelay version  print version, commit, and build time
mailrelay help     print CLI usage
```

Use `--config path` before or after the command, or set `MAILRELAY_CONFIG`.

## Operations and safety

- Dangerous capabilities are disabled until a command explicitly selects them.
- Every request requires both an allowlisted sender and a constant-time token match.
- Unknown configuration fields, duplicate commands, invalid parameter types, relative executables, and unallowlisted HTTP hosts fail startup.
- Configuration reload is parse/validate/build/atomic-swap. Invalid changes are logged and the last valid configuration remains active.
- IMAP prefers IDLE and falls back to bounded polling/reconnect backoff.
- Command execution and SMTP delivery are separated by a durable SQLite outbox. SMTP retry never executes a Handler twice.
- Exhausted queue jobs and replies remain in dead-letter state until an operator uses `mailrelay replay`.
- Logs and SQLite never store tokens, mailbox passwords, API keys, or full mail bodies.
- Back up the YAML file and SQLite database together while MailRelay is stopped, or use SQLite's online backup tooling.

## Development

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/mailrelay
```

The architecture and accepted security boundaries are documented in `docs/superpowers/specs/2026-07-07-mailrelay-design.md`.

## 72-hour acceptance run

Use a dedicated mailbox and one Stable HTTP/Webhook command. Start with an empty queue and outbox:

```bash
mailrelay doctor
mailrelay soak --duration 72h
```

During the run, send valid commands, one unauthorized command, duplicate `Message-ID` messages, and temporarily interrupt IMAP, SMTP, and the target HTTP endpoint. Acceptance requires `soak_result: pass`, no unauthorized or duplicate execution in the audit, and no pending/dead reply or queue item. If a deliberate outage exhausts retries, inspect `mailrelay status`, repair the dependency, and explicitly replay the dead item before restarting the acceptance window.
