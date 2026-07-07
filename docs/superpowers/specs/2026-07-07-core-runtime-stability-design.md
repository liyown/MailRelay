# MailRelay Core Runtime Stability

## Goal

Turn MailRelay's core path into a mature single-host daemon that can run unattended, survive dependency failures, and make every message outcome explainable.

The core path is:

```text
IMAP fetch -> parse -> authenticate -> deduplicate -> execute command -> persist audit and reply -> deliver SMTP reply
```

The stability target is not more handlers. The target is a runtime where one bad message, a temporary network failure, an SMTP outage, or a process restart does not hide work, duplicate execution, or block unrelated messages.

## Scope

This design covers the runtime, store, CLI status/doctor/soak behavior, and tests needed for stable operation.

Included:

- Explicit persisted message processing states.
- Per-message failure isolation in batch processing.
- Durable audit, reply, queue, and failure event records.
- Unified retry policy for queue jobs and SMTP replies.
- Operational visibility through `status`, `doctor`, and `soak`.
- Regression tests for crashes, retry, duplicate suppression, and configuration reload failures.

Excluded:

- Web UI.
- Multi-instance coordination.
- Provider-specific Gmail or Outlook APIs.
- New handler types.
- Expansion of experimental `shell`, `plugin`, `agent`, or `mcp` behavior beyond guardrails needed to protect the stable runtime.

## Design Principles

MailRelay should behave like a conservative operations tool:

- A message is either pending, processing, done, retryable, or dead. It is never silently lost.
- A handler result is never delivered by SMTP before the execution and reply intent are durable.
- Retrying SMTP delivery never re-executes a handler.
- A malformed or unauthorized message cannot stop later messages in the same batch.
- Operator commands explain what happened using local SQLite state, without requiring logs.
- Stable handlers are optimized first; beta and experimental handlers must not weaken the core guarantees.

## Persisted Processing Model

`processed_messages` should become the source of truth for message lifecycle. The stable states are:

- `claimed`: the message was accepted for processing and has not reached a terminal result.
- `auth_failed`: sender or token failed. No command information is exposed.
- `parse_failed`: the MIME or command payload could not be parsed.
- `executing`: a command is currently being executed or the process stopped while it was executing.
- `reply_pending`: execution was recorded and an SMTP reply exists in the outbox.
- `done`: execution and reply delivery completed, or the message required no reply by policy.
- `dead`: processing cannot continue without operator action.

The state row should include safe metadata: message ID, sender, command name when known, current state, attempt counts where relevant, timestamps, and last safe error. It must not store tokens, raw mail bodies, mailbox passwords, API keys, or unredacted sensitive parameters.

On startup, stale `executing` rows are not automatically re-executed. They are surfaced by `status` and can become explicit operator replay work only when the implementation can prove the handler is idempotent or the operator asks for it. This is conservative: avoiding duplicate execution is more important than automatic completion.

## Execution and Reply Flow

The main runtime should process one message as a sequence of durable transitions:

1. Parse raw mail. If parsing fails, record a parse failure tied to UID when possible and continue the batch.
2. Authenticate sender and token. If authentication fails, record `auth_failed` without route details and continue.
3. Claim the message ID. If already claimed or done, skip handler execution and mark the mailbox item seen.
4. Mark the message `executing` before invoking the router.
5. Execute the handler under the configured command timeout and panic recovery.
6. In one store transaction, write the execution audit, redacted parameters, message state, and outbox reply.
7. Deliver the reply through the outbox worker. Delivery failure updates only the outbox row.
8. Mark the mailbox item seen after the message has a durable state that prevents duplicate execution.

This flow preserves the existing durable reply outbox approach, but makes the message lifecycle explicit enough to recover and diagnose interrupted work.

## Batch Failure Isolation

`Once` should treat each fetched IMAP message independently. A failure in one message should not abort the rest of the batch unless the receiver itself is unhealthy.

Message-level failures:

- parse failure
- authentication failure
- unknown command
- invalid parameters
- policy denial
- handler dependency failure
- reply build failure

These should be recorded, then processing should continue with the next message.

Receiver-level failures:

- cannot connect to IMAP
- cannot search or fetch mailbox
- cannot mark a processed UID seen after durable processing

These remain runtime errors and drive reconnect/backoff behavior.

## Retry and Dead-Letter Policy

Queue jobs and SMTP replies should use the same retry model:

- `pending`: ready when `available_at <= now`.
- `running`: leased by one worker until `lease_until`.
- `done`: completed.
- `dead`: attempts exhausted or explicitly stopped.

The retry policy should be configurable with conservative defaults:

- `max_attempts`: default 5 for replies, 3 for queue jobs.
- `initial_backoff`: default 1 minute.
- `max_backoff`: default 30 minutes.
- `jitter`: small positive jitter to avoid repeated synchronized retries.

Replay remains an explicit `dead -> pending` operation. Replay resets lease and scheduling, but it should preserve previous failure history in audit or event records.

## Runtime Events

Add a structured event stream in SQLite. This can be a new `runtime_events` table or an equivalent store abstraction.

Each event records:

- timestamp
- severity
- phase
- message ID when available
- command when available
- handler when available
- safe error kind
- safe summary

Events should cover parse/auth failures, handler failures, reply failures, queue failures, config reload rejection, receiver reconnects, and soak invariant failures.

`status` should use this structured state rather than relying only on one `last_runtime_error` string.

## CLI Behavior

### `status`

`mailrelay status` should report a compact health summary:

- last successful poll
- last runtime error summary
- queue pending/running/dead
- replies pending/running/dead
- stale executing messages
- latest safe failures
- catalog hash and notification state
- latest execution

The command should stay read-only.

### `doctor`

`doctor` should be split conceptually into local checks and network checks:

- Local checks always run: YAML, unknown fields, command validation, handler maturity warnings, SQLite open/migration, executable path safety, storage permissions.
- Network checks should be explicit or clearly labeled: IMAP, SMTP, DNS, and outbound host validation.

This avoids surprising side effects while still supporting deep deployment validation.

### `soak`

`soak` should run the normal runtime and periodically print health snapshots. At completion it fails if core invariants fail:

- unauthorized messages did not execute
- duplicate message IDs did not execute twice
- no pending reply exceeds the configured age threshold
- no queue or reply item is dead unless the operator intentionally caused and accepted it
- IMAP polling has not stalled beyond threshold
- config reload failures preserved the last valid runtime

Live mailbox credentials remain operator supplied and are never stored in tests or CI.

## Configuration Guardrails

Runtime configuration should make stable behavior the default:

- `runtime.config_reload` must be respected. When false, file changes are ignored until restart.
- Stable handlers are `http` and `webhook`.
- Beta handlers are `workflow` and `queue`.
- Experimental handlers are `plugin`, `shell`, `agent`, and `mcp`.
- Experimental handlers should require explicit opt-in, such as `runtime.enable_experimental: true` or a handler allowlist.
- Sensitive parameters must be redacted before any audit, queue, event, or log write.

These guardrails do not remove advanced features. They prevent an operator from enabling risky behavior accidentally.

## Testing Strategy

The stability test suite should prove operational behavior, not just individual functions.

Required tests:

- A malformed message does not stop later valid messages in the same batch.
- An unauthorized message records a safe failure and never reaches the router.
- Duplicate `Message-ID` messages execute at most once.
- Handler success plus SMTP failure leaves a pending reply and retry does not re-execute the handler.
- Exhausted replies and queue jobs become dead and can be explicitly replayed.
- Stale leases become claimable after restart.
- Invalid config reload preserves the last valid router and records a reload event.
- `runtime.config_reload: false` disables hot reload.
- `status` reports queue/reply/dead/stale state from SQLite.
- A short `soak` run fails when invariants are intentionally violated.

Existing unit tests stay valuable, but the acceptance bar for this phase is restart and failure behavior.

## Rollout Plan

Implement in three focused increments:

1. Message lifecycle and batch isolation.
   Add explicit message states, record parse/auth/message failures, and keep processing later messages.

2. Unified recoverability and visibility.
   Add runtime events, strengthen retry/dead-letter queries, and upgrade `status`.

3. Operational hardening.
   Respect config reload settings, split `doctor` checks, strengthen `soak`, and add experimental handler opt-in.

Each increment should include focused failing tests first, then implementation, then full verification with:

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/mailrelay
```

## Acceptance Criteria

This stability phase is complete when:

- A single bad email cannot block unrelated valid email.
- SMTP outage never causes duplicate handler execution.
- Restart after a lease or reply failure leaves work visible and recoverable.
- Operators can determine current health from `mailrelay status`.
- `doctor` catches unsafe configuration before runtime.
- `soak` can run as an operator acceptance check and fail on violated invariants.
- Stable behavior is covered by deterministic local tests, without live credentials in CI.
