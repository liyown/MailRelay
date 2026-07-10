# MailRelay

MailRelay turns authenticated email into a small, configuration-driven command protocol:

```text
Email -> Parser -> Authentication -> Router -> Handler -> Result -> Email reply
```

It is a single-machine Go service. SQLite stores deduplication keys, execution audits, the generated command catalog, runtime health, and queued jobs.

## Five-minute setup

```bash
go install github.com/becomeopc/opc-mailrelay/cmd/mailrelay@latest
mailrelay init
```

Edit `mailrelay.yaml`, replace all `change-me` values, set the sender allowlist, and declare commands. Check the installation before running it:

```bash
mailrelay doctor
mailrelay once
mailrelay run
```

Secrets can be environment references such as `${IMAP_PASSWORD}`. An unresolved reference is rejected. Keep the configuration file readable only by the service account.

## Web console

The optional operations console is embedded in the MailRelay binary and is disabled by default. Generate an Argon2id administrator hash without placing the password in shell history:

```bash
MAILRELAY_ADMIN_PASSWORD='choose-a-strong-password' mailrelay hash-password
```

Store the resulting hash and independent secrets through environment references:

```yaml
web:
  enabled: true
  address: 127.0.0.1:8787
  session_secret: "${MAILRELAY_SESSION_SECRET}"
  admin_password_hash: "${MAILRELAY_ADMIN_PASSWORD_HASH}"
  session_ttl: 8h
```

Open `http://127.0.0.1:8787`. The authenticated console can edit the command catalog, command token, sender allowlist, HTTP host allowlist, and catalog-change notification recipients through the draft configuration API. Its mail simulator checks parsing, sender/token authentication, command routing, and parameter validation without executing a handler, calling an external API, writing an audit record, or sending a reply. The API never returns mailbox credentials, web session secrets, full mail bodies, or raw provider errors. Handler secrets should stay in `${ENV_VAR}` references or `sensitive` parameters. Keep the default loopback binding unless the console is placed behind a trusted HTTPS reverse proxy.

## Sending a command

Send mail from an allowlisted address:

```text
Subject: push
X-MailRelay-Token: your-token

message=hello
```

Plain text `key=value` bodies and JSON object bodies are supported. For JSON, use `Content-Type: application/json`. The reserved `_token` body field can be used when the sending client cannot add headers. A header token takes precedence. `Message-ID` is used for durable deduplication; messages without one receive a deterministic content hash.

Commands using the `http_request` handler are different: the mail body is treated as an HTTP request transcript and forwarded. Use `X-MailRelay-Token` for those messages, because `_token` would be part of the forwarded HTTP body.

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

- **Stable:** `http`, `http_request`, `webhook`, `workflow`, `queue` — supported for the v0.1 golden path and recovery lifecycle.
- **Experimental:** `plugin`, `shell`, `agent`, `mcp`, and custom handlers — APIs and safety limits may still change.

### `http`

Calls a configured HTTP endpoint. The hostname must be listed in `security.http_hosts`. Scheme, credentials, and host must be static; path segments may use `{{param}}` templates, and query parameters should be configured with `query` so values are encoded safely. Resolved private, loopback, link-local, multicast, unspecified, carrier-grade NAT, and metadata addresses are rejected at validation and dial time. Cross-host redirects are rejected.

Configuration keys: `url`, `method`, `headers`, `body`, and `query`.

```yaml
config:
  method: GET
  url: https://api.example.com/push/{{message}}
  query:
    source: mailrelay
```

If `body` is empty, MailRelay does not add a default `Content-Type` header.

### `http_request`

Forwards the mail body as an HTTP/1.1 request. The request line may contain an absolute URL, or an origin-form path when `config.base_url` supplies the scheme and fallback host. The final destination is still checked by `security.http_hosts` and the outbound network policy.

```yaml
commands:
  - name: forward
    handler: http_request
    config:
      base_url: https://api.example.com
```

Example mail body:

```http
POST /events HTTP/1.1
Host: api.example.com
Content-Type: application/json

{"message":"hello"}
```

### `webhook`

POSTs a standard JSON envelope containing version, command, request ID, timestamp, and parameters. `secret` adds an `X-MailRelay-Signature: sha256=...` HMAC header. It uses the same outbound policy as HTTP.

### `workflow`

Runs a bounded list of configured command steps through the Router. Each step has `command` and an explicit `params` mapping for the target command's declared parameters. Values such as `{{env}}` read workflow request parameters; fixed values are passed as-is. Required target parameters must be mapped, unknown target parameters are rejected, and sensitive target parameters must be sourced from sensitive workflow parameters. Missing targets, direct or indirect recursion, excessive depth, timeouts, and excessive step counts fail safely. Execution stops on the first failed step.

### `queue`

Inserts a target command into the SQLite queue with an idempotency key derived from the message. `max_attempts` bounds retry. The local worker leases jobs transactionally and recovers expired leases after restart. Queue wrapper parameters must match the target schema and cannot be sensitive because delayed execution requires their durable plaintext value.

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
mailrelay hash-password  generate the Web console Argon2id password hash
mailrelay help     print CLI usage
```

Use `--config path` before or after the command, or set `MAILRELAY_CONFIG`.

When working from the repository, the Taskfile includes the common development loops:

```bash
task dev            watch files, rebuild the embedded console, and restart via Air
task console-embed  build console/dist and copy it into internal/web/ui
task build          build the binary with Version, Commit, and BuildTime metadata
task test-all       run Go race tests and console tests
```

## Operations and safety

- Dangerous capabilities are disabled until a command explicitly selects them.
- Every request requires both an allowlisted sender and a constant-time token match.
- Unknown configuration fields, duplicate commands, invalid parameter types, relative executables, and unallowlisted HTTP hosts fail startup.
- `mailrelay status` reads SQLite health state and reports queue, reply, dead-letter, stale execution, and recent failure summaries.
- `runtime.config_reload: false` disables hot reload; invalid reloads keep the last valid runtime and are recorded as runtime events.
- Experimental handlers require explicit opt-in and should not be used for the stable HTTP/Webhook golden path.
- Configuration reload is parse/validate/build/atomic-swap. Invalid changes are logged and the last valid configuration remains active.
- IMAP prefers IDLE and falls back to bounded polling/reconnect backoff.
- Command execution and SMTP delivery are separated by a durable SQLite outbox. SMTP retry never executes a Handler twice.
- Exhausted queue jobs and replies remain in dead-letter state until an operator uses `mailrelay replay`.
- SQLite audit records never store tokens, mailbox passwords, API keys, or full mail bodies. Operational logs redact sensitive parameters and include truncated HTTP request/response snapshots for `http`, `http_request`, and `webhook` troubleshooting.
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

Use a dedicated mailbox and one Stable HTTP-family/Webhook command. Start with an empty queue and outbox:

```bash
mailrelay doctor
mailrelay soak --duration 72h
```

During the run, send valid commands, one unauthorized command, duplicate `Message-ID` messages, and temporarily interrupt IMAP, SMTP, and the target HTTP endpoint. Acceptance requires `soak_result: pass`, no unauthorized or duplicate execution in the audit, and no pending/dead reply or queue item. If a deliberate outage exhausts retries, inspect `mailrelay status`, repair the dependency, and explicitly replay the dead item before restarting the acceptance window.
