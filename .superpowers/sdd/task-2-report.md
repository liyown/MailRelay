# Task 2 Report: Runtime Events and Status Health

## Scope

Implemented Task 2 in:

- `internal/store/store.go`
- `internal/store/store_test.go`
- `internal/app/app.go`
- `internal/app/runtime.go`
- `internal/app/runtime_test.go`
- `internal/cli/cli.go`
- `internal/cli/cli_test.go`

No unrelated files were modified for code changes. The report file is separate from the Task 2 commit scope.

## TDD Evidence

### Red

Added failing expectations first:

- `internal/store/store_test.go`: `TestRuntimeEventsAndHealthSummary`
- `internal/cli/cli_test.go`: expanded `TestStatusAndReplayDeadLetters`
- `internal/app/runtime_test.go`: reload failure must persist a runtime event

Executed failing tests:

```bash
go test ./internal/store -run TestRuntimeEventsAndHealthSummary -count=1 -v
go test ./internal/cli -run TestStatusAndReplayDeadLetters -count=1 -v
go test ./internal/app -run TestHotReloadIsAtomicAndKeepsLastValidConfig -count=1 -v
```

Observed failures:

- store package failed to build because `AddEvent`, `RecentEvents`, `Health`, and `RuntimeEvent` did not exist
- CLI status output was missing `queue_pending`, `queue_running`, `reply_running`, `stale_executing`, and `recent_failure`
- app package failed to build because `RecentEvents` did not exist on `Store`

### Green

Implemented:

- `runtime_events` SQLite table and recent-events index
- `store.RuntimeEvent`
- `Store.AddEvent`
- `Store.RecentEvents`
- `store.HealthSummary`
- `Store.Health`
- app/runtime event recording for handler, reply, reload, and receiver failures
- CLI status output backed by `Store.Health` while keeping existing compatibility fields

Executed focused verification:

```bash
go test ./internal/store -run 'TestRuntimeEventsAndHealthSummary|TestQueueDeadLetterReplayAndLatestFailure' -count=1 -v
go test ./internal/app -run 'TestReplyRetryDoesNotExecuteHandlerTwice|TestHotReloadIsAtomicAndKeepsLastValidConfig' -count=1 -v
go test ./internal/cli -run TestStatusAndReplayDeadLetters -count=1 -v
```

All passed.

## Required Final Verification

Executed:

```bash
go test ./internal/store ./internal/app ./internal/cli -count=1
```

Result:

- `ok github.com/becomeopc/opc-mailrelay/internal/store`
- `ok github.com/becomeopc/opc-mailrelay/internal/app`
- `ok github.com/becomeopc/opc-mailrelay/internal/cli`

## Behavioral Notes

- Reply SMTP failures now emit runtime events with phase `reply` and error kind `dependency`.
- Handler execution failures now emit runtime events with phase `handler` and sanitized error classification in `ErrorKind`.
- Reload failures emit runtime events with phase `reload` and error kind `config`.
- `Runtime.Run` now records receiver-loop failures as phase `receiver`.
- CLI `status` now reports queue/reply pending/running/dead counts, stale executing count, and recent runtime failures, while preserving `last_poll`, `runtime_error`, `catalog_hash`, and `last_execution`.

## Privacy / Constraint Check

- No new handler types added.
- No web UI, multi-instance coordination, or provider-specific mailbox APIs added.
- Existing redaction behavior for dead-letter reply persistence remains intact.
- SMTP retry path still only retries reply delivery and does not re-execute handlers.
- Batch processing behavior remains tolerant of malformed/unauthorized messages.

## Commit

Created commit:

- `feat: add runtime events and health status`

---

## Task 2 Fixes: Review Follow-up

Addressed only the review findings against `b2505e6`.

### Findings fixed

1. Runtime event summaries for SMTP reply failures, rejected reloads, and receiver-loop failures are now sanitized and fixed-string classified:
   - `reply` / `dependency` / `reply delivery failed`
   - `reload` / `config` / `configuration reload rejected`
   - `receiver` / `dependency` / `mail receiver failed`
2. Rejected reloads are recorded once per failure when `Run()` calls `reloadIfChanged()`.
3. CLI status test again asserts concrete dead-letter counts while retaining the broader label checks.

### TDD evidence for fixes

#### Red

Added failing coverage first:

- `internal/app/app_test.go`: runtime reply event summary must be sanitized
- `internal/app/runtime_test.go`: reload event summary must be sanitized; `Run()` must not double-record reload failures; receiver runtime event summary must be sanitized
- `internal/cli/cli_test.go`: restore `queue_dead: 1` and `reply_dead: 1` assertions alongside label checks

Observed failing run before implementation:

```bash
go test ./internal/app -run 'TestRunOneReplyRedactsStoredSMTPFailure|TestHotReloadIsAtomicAndKeepsLastValidConfig|TestRunRecordsRejectedReloadOnce|TestRunSanitizesReceiverFailureEvent' -count=1
```

Failures showed:

- raw SMTP text persisted in `runtime_events.summary`
- raw YAML parser text persisted in reload event summary
- reload rejection recorded twice in `Run()`
- receiver runtime event path needed explicit sanitized coverage

#### Green

Implemented minimal fixes in:

- `internal/app/app.go`
- `internal/app/runtime.go`
- `internal/app/runtime_test.go`
- `internal/app/app_test.go`
- `internal/cli/cli_test.go`

Focused verification:

```bash
go test ./internal/store -run 'TestRuntimeEventsAndHealthSummary|TestQueueDeadLetterReplayAndLatestFailure' -count=1 -v
go test ./internal/app -run 'TestReplyRetryDoesNotExecuteHandlerTwice|TestHotReloadIsAtomicAndKeepsLastValidConfig|TestRunRecordsRejectedReloadOnce|TestRunSanitizesReceiverFailureEvent|TestRunOneReplyRedactsStoredSMTPFailure' -count=1 -v
go test ./internal/cli -run TestStatusAndReplayDeadLetters -count=1 -v
```

All passed.

### Required final verification

Executed:

```bash
go test ./internal/store ./internal/app ./internal/cli -count=1
```

Result:

- `ok github.com/becomeopc/opc-mailrelay/internal/store`
- `ok github.com/becomeopc/opc-mailrelay/internal/app`
- `ok github.com/becomeopc/opc-mailrelay/internal/cli`

### Fix commit

Pending commit for review-fix follow-up:

- `fix: sanitize runtime event failures`
