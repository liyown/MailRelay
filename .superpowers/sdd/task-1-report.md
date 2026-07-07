# Task 1 Report: Message Lifecycle and Batch Isolation

Date: 2026-07-07

Scope:
- `internal/store/store.go`
- `internal/store/store_test.go`
- `internal/app/app.go`
- `internal/app/app_test.go`

Summary:
- Added persisted message lifecycle states and lookup/update APIs in the store.
- Extended `processed_messages` migrations in place with `command`, `error_kind`, `error_summary`, and `updated_at`.
- Recorded authentication failures and execution transitions in `App.Process`.
- Changed `App.Once` to isolate per-message failures, record them, continue the batch, and still mark each fetched UID seen.

TDD evidence:
1. Added `TestMessageLifecycleStatesArePersisted` in `internal/store/store_test.go`.
2. Added `TestOnceRecordsBadMessageAndContinuesBatch` in `internal/app/app_test.go`.
3. RED:
   - `go test ./internal/store -run TestMessageLifecycleStatesArePersisted -count=1 -v` failed with missing `MessageUpdate`, `MessageParseFailed`, `RecordMessageFailure`, `MarkMessageExecuting`, and `MessageState`.
   - `go test ./internal/app -run TestOnceRecordsBadMessageAndContinuesBatch -count=1 -v` initially exposed a missing test import, then failed as expected because `Once` returned on the first bad message.
4. GREEN:
   - Implemented the minimal store/app changes to satisfy the new behavior.
   - Re-ran the focused tests successfully.

Verification:
- `go test ./internal/store -run 'TestMessageLifecycleStatesArePersisted|TestClaimsPersistAndQueue|TestReplyOutboxLeaseRetryAndDeadLetter' -count=1 -v`
- `go test ./internal/app -run 'TestOnceRecordsBadMessageAndContinuesBatch|TestProcessAuthenticatesAndDeduplicates|TestReplyRetryDoesNotExecuteHandlerTwice' -count=1 -v`
- `go test ./internal/store ./internal/app -count=1`

Results:
- All required focused tests passed.
- `go test ./internal/store ./internal/app -count=1` passed.

Notes:
- Preserved the pre-existing timestamp-stability edits in `internal/store/store.go` and `internal/store/store_test.go`.
- Did not modify plan docs or unrelated files.

## 2026-07-07 Fix Pass: Review Findings

Scope:
- `internal/store/store.go`
- `internal/store/store_test.go`
- `internal/app/app.go`
- `internal/app/app_test.go`

Fixes applied:
- Malformed messages fetched in `Once` now persist `parse_failed` against the synthetic `uid:<n>` key instead of falling through to a generic `dead` row.
- Batch auth failures no longer create an extra synthetic `uid:<n>` dead row when `Process` already persisted `auth_failed` by normalized `Message-ID`.
- Exhausted SMTP reply retries now transition the related `processed_messages` row to `dead` with `reply_delivery` failure metadata.
- Failure upserts preserve existing sender/command fields when later lifecycle transitions only provide terminal failure metadata.

TDD evidence:
1. Added failing assertions to `TestOnceRecordsBadMessageAndContinuesBatch` for `uid:<n>` `parse_failed` state.
2. Added `TestOnceAuthFailureUsesMessageIDWithoutDeadUIDDuplicate`.
3. Added failing assertions to `TestReplyOutboxLeaseRetryAndDeadLetter` for `MessageDead` transition on exhausted replies.
4. Added `TestMessageFailureUpdatePreservesExistingSenderAndCommand`.
5. RED:
   - `go test ./internal/store -run 'TestReplyOutboxLeaseRetryAndDeadLetter|TestMessageFailureUpdatePreservesExistingSenderAndCommand' -count=1 -v` failed because dead replies did not update `processed_messages`, and failure upserts cleared sender/command.
   - `go test ./internal/app -run 'TestOnceRecordsBadMessageAndContinuesBatch|TestOnceAuthFailureUsesMessageIDWithoutDeadUIDDuplicate' -count=1 -v` failed because malformed batch messages were recorded as generic `dead`, and auth failures did not match the expected message-level lifecycle handling.
6. GREEN:
   - Updated `Once` failure recording to map parse failures to `MessageParseFailed` and skip duplicate UID dead rows for auth failures.
   - Updated `FailReply` to persist terminal reply-delivery failure state into `processed_messages`.
   - Updated `RecordMessageFailure` conflict handling to preserve prior sender/command values when not provided by the new failure update.

Verification:
- `go test ./internal/store -run 'TestMessageLifecycleStatesArePersisted|TestReplyOutboxLeaseRetryAndDeadLetter|TestClaimsPersistAndQueue|TestMessageFailureUpdatePreservesExistingSenderAndCommand' -count=1 -v`
- `go test ./internal/app -run 'TestOnceRecordsBadMessageAndContinuesBatch|TestOnceAuthFailureUsesMessageIDWithoutDeadUIDDuplicate|TestProcessAuthenticatesAndDeduplicates|TestReplyRetryDoesNotExecuteHandlerTwice' -count=1 -v`
- `go test ./internal/store ./internal/app -count=1`

Results:
- All focused fix tests passed.
- Full `internal/store` and `internal/app` package tests passed.
