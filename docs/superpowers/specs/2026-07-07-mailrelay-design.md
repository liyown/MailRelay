# MailRelay Design

## 1. Scope

MailRelay is a single-host, email-driven command runtime distributed as the `mailrelay` CLI from module `github.com/becomeopc/opc-mailrelay`. It converts authenticated email into declarative commands, dispatches those commands through a handler registry, records execution in SQLite, and replies through SMTP.

The product includes IMAP polling and IDLE, SMTP replies, configuration hot reload, HTTP, webhook, workflow, plugin, restricted shell, AI agent, MCP, SQLite queue, and build-time custom handlers. It intentionally excludes a Web UI, multi-instance synchronization, OAuth, and provider-specific Gmail APIs.

## 2. Architecture

```text
IMAP Receiver / IMAP IDLE
        |
        v
MIME Parser
        |
        v
Authentication + Deduplication
        |
        v
Command Router
        |
        v
Handler Registry
        |
        v
HTTP / Webhook / Workflow / Plugin / Shell / Agent / MCP / Queue
        |
        v
Result + Audit Record
        |
        v
SMTP Responder
```

Each boundary has one responsibility:

- Receiver retrieves raw mail and does not know commands or handlers.
- Parser converts mail into a `CommandRequest` and does not execute it.
- Authenticator validates sender and token before discovery or routing.
- Router resolves a configured command and delegates to a named handler.
- Handler Registry owns handler registration and lookup. The Router has no handler-specific branches.
- Responder converts a safe `Result` into a threaded SMTP reply.
- Store owns SQLite transactions, migrations, deduplication, audit records, queue state, and catalog snapshots.

## 3. Core Model

```go
type Command struct {
    Name        string
    Description string
    Handler     string
    Parameters  map[string]Parameter
    Config      map[string]any
}

type Parameter struct {
    Description string
    Type        string
    Required    bool
    Sensitive   bool
    Example     any
}

type CommandRequest struct {
    MessageID string
    Sender    string
    Name      string
    Params    map[string]any
    Received  time.Time
}

type Result struct {
    Status     string
    Summary    string
    Body       string
    Data       map[string]any
    StartedAt  time.Time
    Duration   time.Duration
}

type Handler interface {
    Name() string
    Execute(context.Context, CommandContext) (Result, error)
}
```

Handler interfaces live on the registry/router consumer side. Handler implementations receive normalized commands and requests, never raw email.

## 4. Configuration

MailRelay uses one YAML file. Secret values may use `${ENV_VAR}` and unresolved variables are configuration errors. The stable top-level layout is:

```yaml
mail:
  imap:
    address: imap.example.com:993
    username: relay@example.com
    password: ${IMAP_PASSWORD}
    mailbox: INBOX
    poll_interval: 30s
  smtp:
    address: smtp.example.com:465
    username: relay@example.com
    password: ${SMTP_PASSWORD}
    from: relay@example.com

security:
  token: ${MAILRELAY_TOKEN}
  allow:
    - me@example.com
  http_hosts:
    - api.example.com

storage:
  path: ./data/mailrelay.db

runtime:
  command_timeout: 30s
  config_reload: true
  catalog_notify:
    - admin@example.com

handlers:
  agent:
    endpoint: https://api.example.com/v1
    api_key: ${AGENT_API_KEY}
    model: approved-model
  mcp:
    servers: {}

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
```

Unknown fields and duplicate command names are rejected. Command names use lowercase ASCII letters, digits, `_`, and `-`. `help` is reserved. Configuration reload follows parse, environment expansion, full validation, security validation, then atomic replacement. An invalid replacement leaves the last valid configuration active and records the failure.

## 5. Mail Parsing and Authentication

The Subject is either `<command>` or `help [command]`. Plain text bodies use one `key=value` pair per line. JSON bodies require `Content-Type: application/json` and an object at the top level. `_token` is a reserved body field and `X-MailRelay-Token` is the equivalent mail header. Header wins if both are present.

The parser exposes attachment names, content types, sizes, and content readers, but no handler executes or uploads attachments implicitly.

Before routing, the sender must match the normalized sender allowlist and the token must match in constant time. Authentication failures return no catalog or command details. Parameters are validated against their command definition for required values, primitive type, and unknown names.

`Message-ID` is the deduplication key. If absent, MailRelay hashes normalized sender, subject, body, and received timestamp. Claiming a message and creating its processing record occur in one SQLite transaction, preventing duplicate execution after restart.

## 6. Discovery and Catalog

Discovery is generated exclusively from Command definitions:

- `help` returns all available commands, descriptions, and invocation syntax.
- `help <command>` returns its name, description, parameter definitions, authentication requirement, and generated example.
- Startup and successful configuration reload calculate a canonical catalog and SHA-256 hash.
- SQLite stores the current structured catalog snapshot and hash.
- A semantic comparison produces `Added`, `Removed`, and `Updated` sections.
- When a change exists, MailRelay sends the diff to configured catalog administrators through SMTP and then commits the new notified snapshot.

Help, catalog, diffs, and generated examples read the same in-memory Command values. There is no second command documentation file.

## 7. Handlers

### HTTP

Executes a fixed configured HTTPS URL, method, headers, and request template. URL templates may substitute validated parameters but cannot alter scheme or authority. The client resolves DNS and rejects loopback, private, link-local, multicast, unspecified, and metadata-address destinations. Every new redirect target is checked under the same policy; redirects to a different host are denied. Hosts must appear in `security.http_hosts`.

### Webhook

Builds on the same protected HTTP transport. It sends a fixed JSON envelope containing command name, request ID, timestamp, and configured payload. It supports HMAC-SHA256 signatures using a configured secret. The email cannot select the destination or signature secret.

### Workflow

Executes configured command steps, sequentially or as a dependency DAG. Inputs are explicit parameter mappings. Validation rejects missing targets, cycles, recursive self-calls, excessive depth, and more than the configured maximum steps. A workflow shares a total deadline and returns step summaries without exposing secrets.

### Plugin

Starts a configured executable directly, never through a shell. A versioned JSON request is written to stdin and a bounded JSON result is read from stdout. Executable paths, working directories, environment allowlists, output limits, and timeouts are configuration-controlled. Email parameters cannot select executables or arbitrary environment values.

### Restricted Shell

Starts an explicitly configured executable using an argument array and fixed working directory. Template expansion occurs per argument with no shell parsing. Validation rejects relative executables, writable executable files, unapproved working directories, and undeclared environment variables.

### AI Agent

Calls a configured OpenAI-compatible HTTPS endpoint using fixed model, endpoint, system instructions, timeout, and tool allowlist. Email input supplies only declared prompt parameters. API keys are environment-backed and redacted. Agent tool calls go through the same registered and policy-checked command execution path.

### MCP

Connects only to configured MCP servers and invokes only allowlisted tools with validated argument mappings. The initial transports are stdio and streamable HTTP. Stdio uses fixed executables; HTTP uses the protected outbound network policy. Results are bounded and normalized into `Result`.

### SQLite Queue

Atomically inserts a configured target command, normalized parameters, availability time, attempt count, and idempotency key. A local worker claims jobs transactionally, executes the target through the Router, and records success or retry. Retry count and backoff are bounded. Queue commands cannot enqueue themselves directly or form queue/workflow recursion cycles.

### Custom

Custom handlers are Go implementations registered at build time. There is no runtime `custom` wildcard capable of loading arbitrary code. They use the same Handler interface, validation hooks, timeout, audit, and result normalization.

## 8. SQLite Storage

SQLite runs in WAL mode with foreign keys and a busy timeout. Embedded, ordered migrations create:

- `processed_messages` for deduplication and processing state.
- `executions` for command, handler, safe parameters, status, timing, and safe errors.
- `catalog_snapshots` for canonical catalog JSON, hash, and notification state.
- `queue_jobs` for target command, payload, availability, leases, attempts, and result.
- `runtime_state` for receiver health, last successful poll, and configuration version.

Sensitive parameters are replaced with `[REDACTED]` before persistence. Raw tokens, mailbox passwords, API keys, and full email bodies are never stored.

## 9. Runtime and Mail Adapters

`run` opens SQLite, validates and loads configuration, registers handlers, initializes the catalog, starts the queue worker, and starts IMAP. It prefers IMAP IDLE. On protocol failure or server incompatibility it reconnects with bounded exponential backoff and uses periodic polling. A successful config file change atomically updates commands and reloadable handler settings.

`once` retrieves one bounded batch of unprocessed mail, processes it, runs one bounded queue batch, and exits. SMTP replies include `In-Reply-To` and `References`, use a deterministic status subject, and contain only safe user-facing errors.

## 10. CLI

- `mailrelay init` creates `mailrelay.yaml`, a data directory, and a safe example without real secrets.
- `mailrelay run` runs the long-lived receiver, worker, catalog notifier, and hot reloader.
- `mailrelay once` processes one mail and queue batch.
- `mailrelay status` reads SQLite and reports last receive time, recent execution, queue depth, and catalog hash.
- `mailrelay help` prints CLI usage. Email `help` is the authenticated command catalog.
- `mailrelay doctor` validates configuration, SQLite, DNS, IMAP, SMTP, outbound policy, declared executables, and handler settings without executing commands.

Exit codes are stable: `0` success, `1` runtime failure, and `2` configuration or usage error.

## 11. Errors and Observability

Errors are classified as authentication, parsing, unknown command, invalid parameters, policy denial, timeout, dependency failure, and internal failure. SMTP responses contain classification and safe remediation. Structured local logs and SQLite audits contain request IDs, command, handler, status, and duration, but omit secret values and mail bodies.

Each processing operation has a deadline and panic recovery at the runtime boundary. A Handler failure cannot terminate the receiver loop. SMTP reply failures are recorded and retried without re-executing a completed command.

## 12. Testing and Acceptance

Unit tests cover configuration, environment expansion, parsing, sender/token authentication, parameter validation, routing, catalog rendering and diffs, SQLite migrations and recovery, each handler, URL security policy, redaction, and CLI behavior.

Integration tests use local IMAP, SMTP, HTTP, plugin, agent, and MCP test servers. They prove authenticated mail executes once, unauthorized mail does not reach the Router, replies are threaded, restart deduplication works, queue retries are bounded, catalog changes notify administrators, and invalid hot reload preserves the last valid configuration.

Release verification requires:

```text
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/mailrelay
```

An end-to-end local test must exercise `init`, `doctor`, `once`, `status`, and an authenticated `help` mail without public network access.

## 13. Delivery Stages

The design is delivered as independently testable increments:

1. Core: configuration, SQLite, parser, authentication, Router, HTTP, SMTP, IMAP polling, Help, Catalog, CLI.
2. Event and composition: webhook, workflow, queue, discovery diff notification, hot reload, IMAP IDLE.
3. Controlled local extension: plugin, restricted shell, and custom registration.
4. Remote intelligence: AI agent and MCP handlers.
5. Hardening: full doctor checks, protocol integration tests, race tests, security regression suite, and release documentation.

Each stage leaves a runnable CLI and preserves the same Command and Handler contracts.
