# Core Runtime Stability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make MailRelay's core runtime recoverable, observable, and safe to run unattended as a single-host daemon.

**Architecture:** Store owns durable state transitions for messages, replies, queue jobs, and runtime events. App and Runtime call those store APIs at phase boundaries so one bad message cannot stop a batch and SMTP retry cannot re-execute handlers. CLI commands read SQLite health state and expose conservative operational checks.

**Tech Stack:** Go 1.25+, SQLite via `modernc.org/sqlite`, existing `internal/app`, `internal/store`, `internal/cli`, `internal/config`, IMAP/SMTP abstractions.

## Global Constraints

- Do not add handler types.
- Do not add a Web UI, multi-instance coordination, or provider-specific Gmail/Outlook APIs.
- Stable handlers are `http` and `webhook`.
- Beta handlers are `workflow` and `queue`.
- Experimental handlers are `plugin`, `shell`, `agent`, and `mcp`.
- Tokens, raw mail bodies, mailbox passwords, API keys, and unredacted sensitive parameters must not be stored in logs, SQLite audit, queue rows, event rows, or replies.
- A malformed or unauthorized message must not stop later messages in the same fetched batch.
- SMTP retry must never re-execute a handler.
- `runtime.config_reload` must be respected.
- Live mailbox credentials must not be added to tests or CI.
- The current worktree may already contain uncommitted store timestamp-stability changes in `internal/store/store.go` and `internal/store/store_test.go`; preserve them and build on them rather than reverting.

---

## File Structure

- `internal/store/store.go`: add message lifecycle APIs, runtime event APIs, richer health queries, and retry policy helpers. Keep SQLite formatting helpers local to store.
- `internal/store/store_test.go`: cover state transitions, event recording, stale running counts, retry scheduling, dead-letter replay, and sensitive redaction.
- `internal/app/app.go`: update `Process` and `Once` so message-level failures are recorded and isolated; keep receiver-level failures fatal.
- `internal/app/app_test.go`: test malformed/unauthorized/duplicate/reply-failure behavior through the app API.
- `internal/app/runtime.go`: respect `runtime.config_reload`, record reload/receiver events, and expose runtime health through store.
- `internal/app/runtime_test.go`: test reload disabled and invalid reload event recording.
- `internal/config/config.go`: add runtime stability configuration fields with conservative defaults and experimental-handler opt-in validation.
- `internal/config/config_test.go`: test defaults, reload flag behavior, experimental opt-in, and sensitive parameter validation.
- `internal/cli/cli.go`: upgrade `status`, split `doctor` output into local/network sections, and strengthen `soak` invariant reporting.
- `internal/cli/cli_test.go`: test status health output, doctor local/network behavior, experimental guardrails, and short soak failure.
- `README.md`: update operating guidance after implementation tasks land.

---

### Task 1: Message Lifecycle and Batch Isolation

**Files:**
- Modify: `internal/store/store.go`
- Modify: `internal/store/store_test.go`
- Modify: `internal/app/app.go`
- Modify: `internal/app/app_test.go`

**Interfaces:**
- Consumes: existing `store.Open(path string) (*Store, error)`, `App.Process(ctx context.Context, raw io.ReadCloser) error`, `App.Once(ctx context.Context, r mailbox.Receiver, limit int) error`.
- Produces:
  - `type MessageState string`
  - `const MessageClaimed MessageState = "claimed"` and states `MessageAuthFailed`, `MessageParseFailed`, `MessageExecuting`, `MessageReplyPending`, `MessageDone`, `MessageDead`.
  - `type MessageUpdate struct { ID string; Sender string; Command string; State MessageState; ErrorKind string; ErrorSummary string; ReplyPending bool }`
  - `func (s *Store) RecordMessageFailure(ctx context.Context, u MessageUpdate) error`
  - `func (s *Store) MarkMessageExecuting(ctx context.Context, id, sender, command string) error`
  - `func (s *Store) MessageState(ctx context.Context, id string) (MessageUpdate, error)`

- [ ] **Step 1: Add failing store lifecycle tests**

Add these tests to `internal/store/store_test.go`:

```go
func TestMessageLifecycleStatesArePersisted(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "messages.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err := s.RecordMessageFailure(ctx, MessageUpdate{ID: "parse:1", Sender: "bad@example.com", State: MessageParseFailed, ErrorKind: "parse", ErrorSummary: "missing subject"}); err != nil {
		t.Fatal(err)
	}
	got, err := s.MessageState(ctx, "parse:1")
	if err != nil {
		t.Fatal(err)
	}
	if got.State != MessageParseFailed || got.ErrorKind != "parse" || got.ErrorSummary != "missing subject" {
		t.Fatalf("unexpected state: %#v", got)
	}
	if err := s.MarkMessageExecuting(ctx, "m1", "me@example.com", "push"); err != nil {
		t.Fatal(err)
	}
	got, err = s.MessageState(ctx, "m1")
	if err != nil {
		t.Fatal(err)
	}
	if got.State != MessageExecuting || got.Command != "push" || got.Sender != "me@example.com" {
		t.Fatalf("unexpected executing state: %#v", got)
	}
}
```

Add this test to `internal/app/app_test.go`:

```go
type batchReceiver struct {
	msgs []mailbox.RawMessage
	seen []uint32
}

func (r *batchReceiver) Fetch(context.Context, int) ([]mailbox.RawMessage, error) { return r.msgs, nil }
func (r *batchReceiver) MarkSeen(_ context.Context, uid uint32) error {
	r.seen = append(r.seen, uid)
	return nil
}
func (r *batchReceiver) Idle(context.Context) error { return nil }

func TestOnceRecordsBadMessageAndContinuesBatch(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/batch.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h := &testHandler{}
	reg := handler.NewRegistry()
	_ = reg.Register(h)
	route, _ := router.New([]command.Command{{Name: "push", Handler: "test"}}, reg)
	sender := &testSender{}
	a := New(st, route, sender, "relay@example.com", []string{"me@example.com"}, "secret")
	recv := &batchReceiver{msgs: []mailbox.RawMessage{
		{UID: 1, Body: []byte("not a valid message")},
		{UID: 2, Body: []byte("From: me@example.com\r\nSubject: push\r\nMessage-ID: <good-batch>\r\nX-MailRelay-Token: secret\r\n\r\n")},
	}}
	if err := a.Once(context.Background(), recv, 100); err != nil {
		t.Fatal(err)
	}
	if h.calls != 1 || sender.n != 1 {
		t.Fatalf("handler calls=%d sends=%d", h.calls, sender.n)
	}
	if len(recv.seen) != 2 || recv.seen[0] != 1 || recv.seen[1] != 2 {
		t.Fatalf("seen=%v", recv.seen)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/store -run TestMessageLifecycleStatesArePersisted -count=1 -v
go test ./internal/app -run TestOnceRecordsBadMessageAndContinuesBatch -count=1 -v
```

Expected: FAIL because `MessageUpdate`, `MessageParseFailed`, `MarkMessageExecuting`, and batch failure isolation do not exist yet.

- [ ] **Step 3: Implement store message lifecycle APIs**

In `internal/store/store.go`, add state types near the existing store types:

```go
type MessageState string

const (
	MessageClaimed      MessageState = "claimed"
	MessageAuthFailed   MessageState = "auth_failed"
	MessageParseFailed  MessageState = "parse_failed"
	MessageExecuting    MessageState = "executing"
	MessageReplyPending MessageState = "reply_pending"
	MessageDone         MessageState = "done"
	MessageDead         MessageState = "dead"
)

type MessageUpdate struct {
	ID           string
	Sender       string
	Command      string
	State        MessageState
	ErrorKind    string
	ErrorSummary string
	ReplyPending bool
}
```

Extend `processed_messages` migration without dropping existing data:

```sql
ALTER TABLE processed_messages ADD COLUMN command TEXT NOT NULL DEFAULT '';
ALTER TABLE processed_messages ADD COLUMN error_kind TEXT NOT NULL DEFAULT '';
ALTER TABLE processed_messages ADD COLUMN error_summary TEXT NOT NULL DEFAULT '';
ALTER TABLE processed_messages ADD COLUMN updated_at TEXT NOT NULL DEFAULT '';
```

Because SQLite returns an error if a column already exists, implement a helper:

```go
func (s *Store) addColumn(table, column, definition string) error {
	_, err := s.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition))
	if err != nil && strings.Contains(err.Error(), "duplicate column name") {
		return nil
	}
	return err
}
```

Add the lifecycle methods:

```go
func (s *Store) RecordMessageFailure(ctx context.Context, u MessageUpdate) error {
	if u.ID == "" {
		u.ID = "generated:" + dbTime(time.Now()) + ":" + u.State.String()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO processed_messages(id,sender,status,reply_pending,command,error_kind,error_summary,updated_at)
VALUES(?,?,?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET sender=excluded.sender,status=excluded.status,reply_pending=excluded.reply_pending,command=excluded.command,error_kind=excluded.error_kind,error_summary=excluded.error_summary,updated_at=excluded.updated_at`,
		u.ID, u.Sender, string(u.State), boolInt(u.ReplyPending), u.Command, u.ErrorKind, u.ErrorSummary, dbTime(time.Now()))
	return err
}

func (s *Store) MarkMessageExecuting(ctx context.Context, id, sender, command string) error {
	return s.RecordMessageFailure(ctx, MessageUpdate{ID: id, Sender: sender, Command: command, State: MessageExecuting})
}

func (s *Store) MessageState(ctx context.Context, id string) (MessageUpdate, error) {
	var u MessageUpdate
	var state string
	var pending int
	err := s.db.QueryRowContext(ctx, `SELECT id,sender,COALESCE(command,''),status,COALESCE(error_kind,''),COALESCE(error_summary,''),reply_pending FROM processed_messages WHERE id=?`, id).Scan(&u.ID, &u.Sender, &u.Command, &state, &u.ErrorKind, &u.ErrorSummary, &pending)
	u.State = MessageState(state)
	u.ReplyPending = pending != 0
	return u, err
}
```

Add:

```go
func (s MessageState) String() string { return string(s) }

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
```

Update existing `ClaimMessage`, `MarkMessage`, `RecordExecutionAndReply`, and `CompleteReply` to write `updated_at`, `command`, `error_kind`, and message states consistently:

```go
INSERT OR IGNORE INTO processed_messages(id,sender,status,updated_at) VALUES(?,?,?,?)
```

Use `MessageClaimed`, `MessageReplyPending`, and `MessageDone`.

- [ ] **Step 4: Implement app batch isolation**

In `internal/app/app.go`, change `Once` so it records message-level failures and continues:

```go
func (a *App) Once(ctx context.Context, r mailbox.Receiver, limit int) error {
	msgs, err := r.Fetch(ctx, limit)
	if err != nil {
		return err
	}
	for _, m := range msgs {
		err = a.Process(ctx, mailbox.RawReader(m))
		if err != nil {
			_ = a.store.RecordMessageFailure(ctx, store.MessageUpdate{
				ID:           fmt.Sprintf("uid:%d", m.UID),
				State:        store.MessageDead,
				ErrorKind:    "message",
				ErrorSummary: classify(err),
			})
		}
		if markErr := r.MarkSeen(ctx, m.UID); markErr != nil {
			return markErr
		}
	}
	return nil
}
```

In `Process`, after authentication and claim, call:

```go
if err = a.store.MarkMessageExecuting(ctx, m.Request.MessageID, m.Request.Sender, m.Request.Name); err != nil {
	return err
}
```

When authentication fails, record:

```go
_ = a.store.RecordMessageFailure(ctx, store.MessageUpdate{
	ID:           m.Request.MessageID,
	Sender:       m.Request.Sender,
	State:        store.MessageAuthFailed,
	ErrorKind:    "authentication",
	ErrorSummary: "authentication failed",
})
```

Keep returning the authentication error so direct `Process` callers can detect it; `Once` handles it as an isolated message-level failure.

- [ ] **Step 5: Run focused tests**

Run:

```bash
go test ./internal/store -run 'TestMessageLifecycleStatesArePersisted|TestClaimsPersistAndQueue|TestReplyOutboxLeaseRetryAndDeadLetter' -count=1 -v
go test ./internal/app -run 'TestOnceRecordsBadMessageAndContinuesBatch|TestProcessAuthenticatesAndDeduplicates|TestReplyRetryDoesNotExecuteHandlerTwice' -count=1 -v
```

Expected: PASS.

- [ ] **Step 6: Commit Task 1**

Run:

```bash
git add internal/store/store.go internal/store/store_test.go internal/app/app.go internal/app/app_test.go
git commit -m "feat: persist message lifecycle states"
```

---

### Task 2: Runtime Events and Status Health

**Files:**
- Modify: `internal/store/store.go`
- Modify: `internal/store/store_test.go`
- Modify: `internal/app/app.go`
- Modify: `internal/app/runtime.go`
- Modify: `internal/app/runtime_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`

**Interfaces:**
- Consumes: Task 1 `MessageUpdate`, message states, existing reply and queue state tables.
- Produces:
  - `type RuntimeEvent struct { ID int64; At time.Time; Severity string; Phase string; MessageID string; Command string; Handler string; ErrorKind string; Summary string }`
  - `func (s *Store) AddEvent(ctx context.Context, e RuntimeEvent) error`
  - `func (s *Store) RecentEvents(ctx context.Context, limit int) ([]RuntimeEvent, error)`
  - `type HealthSummary struct { QueuePending int; QueueRunning int; QueueDead int; ReplyPending int; ReplyRunning int; ReplyDead int; StaleExecuting int; LatestFailures []RuntimeEvent }`
  - `func (s *Store) Health(ctx context.Context) (HealthSummary, error)`

- [ ] **Step 1: Add failing store event and health tests**

Add to `internal/store/store_test.go`:

```go
func TestRuntimeEventsAndHealthSummary(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "health.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err := s.AddEvent(ctx, RuntimeEvent{Severity: "error", Phase: "reload", ErrorKind: "config", Summary: "invalid yaml"}); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkMessageExecuting(ctx, "stale-1", "me@example.com", "push"); err != nil {
		t.Fatal(err)
	}
	events, err := s.RecentEvents(ctx, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Phase != "reload" || events[0].Summary != "invalid yaml" {
		t.Fatalf("events=%#v", events)
	}
	health, err := s.Health(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if health.StaleExecuting != 1 || len(health.LatestFailures) != 1 {
		t.Fatalf("health=%#v", health)
	}
}
```

Update `internal/cli/cli_test.go` `TestStatusAndReplayDeadLetters` expected strings to include:

```go
for _, want := range []string{"queue_pending:", "queue_running:", "reply_running:", "stale_executing:", "recent_failure:"} {
	if !strings.Contains(out.String(), want) {
		t.Fatalf("missing %q in %s", want, out.String())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/store -run TestRuntimeEventsAndHealthSummary -count=1 -v
go test ./internal/cli -run TestStatusAndReplayDeadLetters -count=1 -v
```

Expected: FAIL because runtime events and richer health output do not exist.

- [ ] **Step 3: Implement runtime event storage**

In `internal/store/store.go`, extend migration:

```sql
CREATE TABLE IF NOT EXISTS runtime_events(
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	at TEXT NOT NULL,
	severity TEXT NOT NULL,
	phase TEXT NOT NULL,
	message_id TEXT NOT NULL DEFAULT '',
	command TEXT NOT NULL DEFAULT '',
	handler TEXT NOT NULL DEFAULT '',
	error_kind TEXT NOT NULL DEFAULT '',
	summary TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS runtime_events_recent ON runtime_events(id DESC);
```

Add types and methods:

```go
type RuntimeEvent struct {
	ID         int64
	At         time.Time
	Severity   string
	Phase      string
	MessageID  string
	Command    string
	Handler    string
	ErrorKind  string
	Summary    string
}

func (s *Store) AddEvent(ctx context.Context, e RuntimeEvent) error {
	if e.At.IsZero() {
		e.At = time.Now()
	}
	if e.Severity == "" {
		e.Severity = "info"
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO runtime_events(at,severity,phase,message_id,command,handler,error_kind,summary) VALUES(?,?,?,?,?,?,?,?)`, dbTime(e.At), e.Severity, e.Phase, e.MessageID, e.Command, e.Handler, e.ErrorKind, e.Summary)
	return err
}

func (s *Store) RecentEvents(ctx context.Context, limit int) ([]RuntimeEvent, error) {
	if limit < 1 {
		limit = 1
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,at,severity,phase,message_id,command,handler,error_kind,summary FROM runtime_events ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RuntimeEvent
	for rows.Next() {
		var e RuntimeEvent
		var at string
		if err := rows.Scan(&e.ID, &at, &e.Severity, &e.Phase, &e.MessageID, &e.Command, &e.Handler, &e.ErrorKind, &e.Summary); err != nil {
			return nil, err
		}
		e.At, _ = parseDBTime(at)
		out = append(out, e)
	}
	return out, rows.Err()
}
```

Add health helpers:

```go
type HealthSummary struct {
	QueuePending   int
	QueueRunning   int
	QueueDead      int
	ReplyPending   int
	ReplyRunning   int
	ReplyDead      int
	StaleExecuting int
	LatestFailures []RuntimeEvent
}

func (s *Store) Health(ctx context.Context) (HealthSummary, error) {
	var h HealthSummary
	err := s.db.QueryRowContext(ctx, `SELECT
		(SELECT count(*) FROM queue_jobs WHERE status='pending'),
		(SELECT count(*) FROM queue_jobs WHERE status='running'),
		(SELECT count(*) FROM queue_jobs WHERE status='dead'),
		(SELECT count(*) FROM outbox_replies WHERE status='pending'),
		(SELECT count(*) FROM outbox_replies WHERE status='running'),
		(SELECT count(*) FROM outbox_replies WHERE status='dead'),
		(SELECT count(*) FROM processed_messages WHERE status='executing')`).Scan(&h.QueuePending, &h.QueueRunning, &h.QueueDead, &h.ReplyPending, &h.ReplyRunning, &h.ReplyDead, &h.StaleExecuting)
	if err != nil {
		return h, err
	}
	h.LatestFailures, err = s.RecentEvents(ctx, 5)
	return h, err
}
```

- [ ] **Step 4: Record app and runtime events**

In `internal/app/app.go`, when `Process` converts a handler error into a safe result, also call:

```go
_ = a.store.AddEvent(ctx, store.RuntimeEvent{Severity: "error", Phase: "handler", MessageID: m.Request.MessageID, Command: m.Request.Name, Handler: handlerName, ErrorKind: safeErr, Summary: "handler failed"})
```

When `deliverReply` or `RunOneReply` sees SMTP failure, call:

```go
_ = a.store.AddEvent(ctx, store.RuntimeEvent{Severity: "error", Phase: "reply", MessageID: r.MessageID, ErrorKind: "dependency", Summary: err.Error()})
```

In `internal/app/runtime.go`, when reload is rejected, call:

```go
_ = r.store.AddEvent(ctx, store.RuntimeEvent{Severity: "error", Phase: "reload", ErrorKind: "config", Summary: err.Error()})
```

When `Run` catches `Once` failure, call:

```go
_ = r.store.AddEvent(context.Background(), store.RuntimeEvent{Severity: "error", Phase: "receiver", ErrorKind: "dependency", Summary: err.Error()})
```

- [ ] **Step 5: Upgrade status output**

In `internal/cli/cli.go` `status`, replace separate queue/reply/dead count printing with `s.Health(ctx)`:

```go
h, err := s.Health(ctx)
if err != nil {
	return err
}
fmt.Fprintf(out, "queue_pending: %d\nqueue_running: %d\nqueue_dead: %d\n", h.QueuePending, h.QueueRunning, h.QueueDead)
fmt.Fprintf(out, "reply_pending: %d\nreply_running: %d\nreply_dead: %d\n", h.ReplyPending, h.ReplyRunning, h.ReplyDead)
fmt.Fprintf(out, "stale_executing: %d\n", h.StaleExecuting)
if len(h.LatestFailures) == 0 {
	fmt.Fprintln(out, "recent_failure: none")
} else {
	for _, e := range h.LatestFailures {
		fmt.Fprintf(out, "recent_failure: %s %s %s\n", e.Phase, e.ErrorKind, e.Summary)
	}
}
```

Keep existing `last_poll`, `runtime_error`, `catalog_hash`, and `last_execution` output for compatibility.

- [ ] **Step 6: Run focused tests**

Run:

```bash
go test ./internal/store -run 'TestRuntimeEventsAndHealthSummary|TestQueueDeadLetterReplayAndLatestFailure' -count=1 -v
go test ./internal/app -run 'TestReplyRetryDoesNotExecuteHandlerTwice|TestHotReloadIsAtomicAndKeepsLastValidConfig' -count=1 -v
go test ./internal/cli -run TestStatusAndReplayDeadLetters -count=1 -v
```

Expected: PASS.

- [ ] **Step 7: Commit Task 2**

Run:

```bash
git add internal/store/store.go internal/store/store_test.go internal/app/app.go internal/app/runtime.go internal/app/runtime_test.go internal/cli/cli.go internal/cli/cli_test.go
git commit -m "feat: add runtime events and health status"
```

---

### Task 3: Operational Guardrails, Doctor, and Soak

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/app/runtime.go`
- Modify: `internal/app/runtime_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`
- Modify: `README.md`

**Interfaces:**
- Consumes: Task 2 `Store.Health` and runtime events.
- Produces:
  - `Runtime.EnableExperimental bool`
  - `Runtime.ReplyMaxAttempts int`
  - `Runtime.QueueMaxAttempts int`
  - `Runtime.InitialBackoff time.Duration`
  - `Runtime.MaxBackoff time.Duration`

- [ ] **Step 1: Add failing config guardrail tests**

Add to `internal/config/config_test.go`:

```go
func TestExperimentalHandlersRequireOptIn(t *testing.T) {
	p := filepath.Join(t.TempDir(), "mailrelay.yaml")
	body := `mail:
  imap: {address: "imap.example.com:993", username: relay, password: pass}
  smtp: {address: "smtp.example.com:465", username: relay, password: pass, from: relay@example.com}
security: {token: secret, allow: [me@example.com]}
runtime: {enable_experimental: false}
commands:
  - name: local
    handler: shell
    config: {executable: /bin/echo}
`
	if err := os.WriteFile(p, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil || !strings.Contains(err.Error(), "experimental handler") {
		t.Fatalf("expected experimental handler error, got %v", err)
	}
	body = strings.Replace(body, "enable_experimental: false", "enable_experimental: true", 1)
	if err := os.WriteFile(p, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeRetryDefaults(t *testing.T) {
	p := filepath.Join(t.TempDir(), "mailrelay.yaml")
	body := `mail:
  imap: {address: "imap.example.com:993", username: relay, password: pass}
  smtp: {address: "smtp.example.com:465", username: relay, password: pass, from: relay@example.com}
security: {token: secret, allow: [me@example.com]}
commands: []
`
	if err := os.WriteFile(p, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Runtime.ReplyMaxAttempts != 5 || c.Runtime.QueueMaxAttempts != 3 || c.Runtime.InitialBackoff != time.Minute || c.Runtime.MaxBackoff != 30*time.Minute {
		t.Fatalf("unexpected runtime defaults: %#v", c.Runtime)
	}
}
```

Add `time` import to `internal/config/config_test.go`.

- [ ] **Step 2: Add failing runtime reload-disabled test**

Add to `internal/app/runtime_test.go`:

```go
func TestConfigReloadFalseIgnoresFileChanges(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "mailrelay.yaml")
	writeConfig := func(desc string) {
		body := `mail:
  imap: {address: "imap.example.com:993", username: u, password: p}
  smtp: {address: "smtp.example.com:465", username: u, password: p, from: relay@example.com}
security: {token: secret, allow: [me@example.com], http_hosts: [api.example.com]}
storage: {path: relay.db}
runtime: {command_timeout: 1s, config_reload: false}
commands:
  - name: push
    description: ` + desc + `
    handler: http
    config: {url: "https://api.example.com/push"}
`
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	writeConfig("old")
	r, err := Build(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	writeConfig("new")
	future := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(path, future, future)
	if err := r.reloadIfChanged(context.Background()); err != nil {
		t.Fatal(err)
	}
	res, err := r.app.Execute(context.Background(), command.Request{Name: "help", Params: map[string]any{"command": "push"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res.Body, "new") || !strings.Contains(res.Body, "old") {
		t.Fatalf("reload should be disabled, body=%q", res.Body)
	}
}
```

- [ ] **Step 3: Add failing CLI doctor and soak tests**

In `internal/cli/cli_test.go`, add:

```go
func TestDoctorLabelsLocalChecksAndSkipsNetworkByDefault(t *testing.T) {
	d := t.TempDir()
	cfg := filepath.Join(d, "mailrelay.yaml")
	var out, errout bytes.Buffer
	if code := Run(context.Background(), []string{"--config", cfg, "init"}, &out, &errout); code != 0 {
		t.Fatal(errout.String())
	}
	out.Reset()
	errout.Reset()
	if code := Run(context.Background(), []string{"--config", cfg, "doctor"}, &out, &errout); code != 0 {
		t.Fatal(errout.String())
	}
	for _, want := range []string{"local checks:", "network checks: skipped"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in %s", want, out.String())
		}
	}
}
```

Update `TestVersionAndZeroDurationSoak` to expect periodic health fields:

```go
for _, want := range []string{"queue_dead:", "reply_dead:", "reply_pending:", "soak_result: pass"} {
	if !strings.Contains(out.String(), want) {
		t.Fatalf("missing %q in %s", want, out.String())
	}
}
```

- [ ] **Step 4: Run tests to verify they fail**

Run:

```bash
go test ./internal/config -run 'TestExperimentalHandlersRequireOptIn|TestRuntimeRetryDefaults' -count=1 -v
go test ./internal/app -run TestConfigReloadFalseIgnoresFileChanges -count=1 -v
go test ./internal/cli -run 'TestDoctorLabelsLocalChecksAndSkipsNetworkByDefault|TestVersionAndZeroDurationSoak' -count=1 -v
```

Expected: FAIL because the new config fields and behavior do not exist.

- [ ] **Step 5: Implement runtime config fields and defaults**

In `internal/config/config.go`, extend `Runtime`:

```go
type Runtime struct {
	CommandTimeout     string        `yaml:"command_timeout"`
	ConfigReload       bool          `yaml:"config_reload"`
	CatalogNotify      []string      `yaml:"catalog_notify"`
	EnableExperimental bool          `yaml:"enable_experimental"`
	ReplyMaxAttempts   int           `yaml:"reply_max_attempts"`
	QueueMaxAttempts   int           `yaml:"queue_max_attempts"`
	InitialBackoff     time.Duration `yaml:"-"`
	MaxBackoff         time.Duration `yaml:"-"`
}
```

Because durations need string parsing, add a custom raw runtime decode in `Load` after YAML decode or add `UnmarshalYAML` for `Runtime`:

```go
func (r *Runtime) UnmarshalYAML(n *yaml.Node) error {
	type raw struct {
		CommandTimeout     string   `yaml:"command_timeout"`
		ConfigReload       *bool    `yaml:"config_reload"`
		CatalogNotify      []string `yaml:"catalog_notify"`
		EnableExperimental bool     `yaml:"enable_experimental"`
		ReplyMaxAttempts   int      `yaml:"reply_max_attempts"`
		QueueMaxAttempts   int      `yaml:"queue_max_attempts"`
		InitialBackoff     string   `yaml:"initial_backoff"`
		MaxBackoff         string   `yaml:"max_backoff"`
	}
	var x raw
	if err := n.Decode(&x); err != nil {
		return err
	}
	r.CommandTimeout = x.CommandTimeout
	if x.ConfigReload != nil {
		r.ConfigReload = *x.ConfigReload
	} else {
		r.ConfigReload = true
	}
	r.CatalogNotify = x.CatalogNotify
	r.EnableExperimental = x.EnableExperimental
	r.ReplyMaxAttempts = x.ReplyMaxAttempts
	r.QueueMaxAttempts = x.QueueMaxAttempts
	if x.InitialBackoff != "" {
		d, err := time.ParseDuration(x.InitialBackoff)
		if err != nil {
			return err
		}
		r.InitialBackoff = d
	}
	if x.MaxBackoff != "" {
		d, err := time.ParseDuration(x.MaxBackoff)
		if err != nil {
			return err
		}
		r.MaxBackoff = d
	}
	return nil
}
```

After decode defaults in `Load`:

```go
if c.Runtime.ReplyMaxAttempts == 0 {
	c.Runtime.ReplyMaxAttempts = 5
}
if c.Runtime.QueueMaxAttempts == 0 {
	c.Runtime.QueueMaxAttempts = 3
}
if c.Runtime.InitialBackoff == 0 {
	c.Runtime.InitialBackoff = time.Minute
}
if c.Runtime.MaxBackoff == 0 {
	c.Runtime.MaxBackoff = 30 * time.Minute
}
```

In `Validate`, reject experimental handlers unless enabled:

```go
if command.HandlerMaturity(cmd.Handler) == "Experimental" && !c.Runtime.EnableExperimental {
	return fmt.Errorf("experimental handler %q requires runtime.enable_experimental", cmd.Handler)
}
```

- [ ] **Step 6: Respect config reload flag**

In `internal/app/runtime.go`, at the start of `reloadIfChanged`:

```go
r.mu.RLock()
reload := r.cfg.Runtime.ConfigReload
r.mu.RUnlock()
if !reload {
	return nil
}
```

Keep invalid reload behavior from existing tests.

- [ ] **Step 7: Split doctor local and network output**

In `internal/cli/cli.go`, modify `doctor` output:

```go
fmt.Fprintln(out, "local checks:")
fmt.Fprintln(out, "configuration: ok")
...
fmt.Fprintln(out, "network checks: skipped")
```

Keep current host policy checks as local DNS/policy checks only if they do not dial remote services. Do not connect to IMAP/SMTP in default doctor.

- [ ] **Step 8: Strengthen soak output with health summary**

In `soak`, after `r.Run(ctx)`, use `r.Store().Health(context.Background())` and print:

```go
fmt.Fprintf(out, "queue_dead: %d\nreply_dead: %d\nreply_pending: %d\nstale_executing: %d\n", h.QueueDead, h.ReplyDead, h.ReplyPending+h.ReplyRunning, h.StaleExecuting)
```

Fail if any of `h.QueueDead`, `h.ReplyDead`, `h.ReplyPending+h.ReplyRunning`, or `h.StaleExecuting` is non-zero.

- [ ] **Step 9: Update README operations guidance**

In `README.md`, update operations bullets to include:

```markdown
- `mailrelay status` reads SQLite health state and reports queue, reply, dead-letter, stale execution, and recent failure summaries.
- `runtime.config_reload: false` disables hot reload; invalid reloads keep the last valid runtime and are recorded as runtime events.
- Experimental handlers require explicit opt-in and should not be used for the stable HTTP/Webhook golden path.
```

- [ ] **Step 10: Run focused tests**

Run:

```bash
go test ./internal/config -count=1
go test ./internal/app -run 'TestHotReloadIsAtomicAndKeepsLastValidConfig|TestConfigReloadFalseIgnoresFileChanges' -count=1 -v
go test ./internal/cli -run 'TestDoctorLabelsLocalChecksAndSkipsNetworkByDefault|TestVersionAndZeroDurationSoak|TestDoctorWarnsForExperimentalHandler' -count=1 -v
```

Expected: PASS. If `TestDoctorWarnsForExperimentalHandler` now fails because experimental handlers require opt-in, update that test config to include `runtime.enable_experimental: true`.

- [ ] **Step 11: Commit Task 3**

Run:

```bash
git add internal/config/config.go internal/config/config_test.go internal/app/runtime.go internal/app/runtime_test.go internal/cli/cli.go internal/cli/cli_test.go README.md
git commit -m "feat: add runtime operational guardrails"
```

---

### Task 4: Final Verification and Stability Audit

**Files:**
- Modify only if verification exposes issues:
  - `internal/store/store.go`
  - `internal/app/app.go`
  - `internal/app/runtime.go`
  - `internal/cli/cli.go`
  - `internal/config/config.go`
  - tests in matching packages

**Interfaces:**
- Consumes: Tasks 1-3 completed and committed.
- Produces: Verified branch with stability design requirements checked against code and tests.

- [ ] **Step 1: Run formatting**

Run:

```bash
gofmt -w internal/store/store.go internal/store/store_test.go internal/app/app.go internal/app/app_test.go internal/app/runtime.go internal/app/runtime_test.go internal/config/config.go internal/config/config_test.go internal/cli/cli.go internal/cli/cli_test.go
```

Expected: no output.

- [ ] **Step 2: Run full unit test suite**

Run:

```bash
go test ./... -count=1
```

Expected: every package reports `ok` or `[no test files]`.

- [ ] **Step 3: Run race test suite**

Run:

```bash
go test -race ./...
```

Expected: every package reports `ok` or `[no test files]`; no data race reports.

- [ ] **Step 4: Run vet**

Run:

```bash
go vet ./...
```

Expected: exit 0 with no diagnostics.

- [ ] **Step 5: Run build**

Run:

```bash
go build ./cmd/mailrelay
```

Expected: exit 0 and a `mailrelay` binary in the repository root or command output location.

- [ ] **Step 6: Run diff checks**

Run:

```bash
git diff --check
git status --short
```

Expected: `git diff --check` exits 0. `git status --short` only shows intended files if verification fixes were needed.

- [ ] **Step 7: Audit design coverage**

Read `docs/superpowers/specs/2026-07-07-core-runtime-stability-design.md` and confirm each acceptance criterion has code and tests:

```text
- A single bad email cannot block unrelated valid email.
- SMTP outage never causes duplicate handler execution.
- Restart after a lease or reply failure leaves work visible and recoverable.
- Operators can determine current health from mailrelay status.
- doctor catches unsafe configuration before runtime.
- soak can run as an operator acceptance check and fail on violated invariants.
- Stable behavior is covered by deterministic local tests, without live credentials in CI.
```

If any criterion lacks test coverage, add the smallest missing test in the relevant package, run it red, implement the fix, and rerun Steps 1-6.

- [ ] **Step 8: Commit verification fixes if any**

If Step 7 required changes, run:

```bash
git add internal README.md docs/superpowers/specs/2026-07-07-core-runtime-stability-design.md
git commit -m "test: complete core runtime stability coverage"
```

If no changes were needed, do not create an empty commit.
