package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Store struct{ db *sql.DB }

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

type Execution struct {
	ID                                                  int64
	MessageID, Command, Handler, Status, Summary, Error string
	StartedAt                                           time.Time
	Duration                                            time.Duration
}
type Job struct {
	ID                    int64
	Command               string
	Params                map[string]any
	Attempts, MaxAttempts int
	AvailableAt           time.Time
}
type Reply struct {
	ID                    int64
	MessageID, Recipient  string
	Payload               []byte
	Attempts, MaxAttempts int
	AvailableAt           time.Time
}

type RuntimeEvent struct {
	ID        int64
	At        time.Time
	Severity  string
	Phase     string
	MessageID string
	Command   string
	Handler   string
	ErrorKind string
	Summary   string
}

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

type ExecutionRecord struct {
	ID                                                  int64
	MessageID, Command, Handler, Status, Summary, Error string
	Sender                                              string
	StartedAt                                           time.Time
	Duration                                            time.Duration
}

type JobRecord struct {
	ID                    int64
	Command, Status       string
	Attempts, MaxAttempts int
	AvailableAt           time.Time
}

type ReplyRecord struct {
	ID                     int64
	MessageID, Recipient   string
	Status, LastError      string
	Attempts, MaxAttempts  int
	AvailableAt, CreatedAt time.Time
}

type ConsoleCounts struct {
	Executions, Success, ActiveHandlers   int
	QueuePending, QueueRunning, QueueDead int
	ReplyPending, ReplyRunning, ReplyDead int
}

type SeriesPoint struct {
	BucketStart    time.Time
	Count, Success int
}

const dbTimeLayout = "2006-01-02T15:04:05.000000000Z07:00"

func dbTime(t time.Time) string {
	return t.UTC().Format(dbTimeLayout)
}

func parseDBTime(v string) (time.Time, error) {
	if t, err := time.Parse(dbTimeLayout, v); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339Nano, v)
}

func (s MessageState) String() string { return string(s) }

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func safeReplyFailureReason(string) string {
	return "delivery failed"
}

func safeExpiredLeaseReason(kind string) string {
	if kind == "queue" {
		return "lease expired after final attempt"
	}
	return safeReplyFailureReason("")
}

func safeQueueFailureReason(string) string {
	return "dependency"
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	s := &Store{db}
	if err = s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) migrate() error {
	_, err := s.db.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON; PRAGMA busy_timeout=5000;
CREATE TABLE IF NOT EXISTS processed_messages(id TEXT PRIMARY KEY,sender TEXT NOT NULL,status TEXT NOT NULL DEFAULT 'claimed',reply_pending INTEGER NOT NULL DEFAULT 0,created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS executions(id INTEGER PRIMARY KEY AUTOINCREMENT,message_id TEXT,command TEXT NOT NULL,handler TEXT NOT NULL,params_json TEXT NOT NULL,status TEXT NOT NULL,summary TEXT,error TEXT,started_at TEXT NOT NULL,duration_ms INTEGER NOT NULL DEFAULT 0);
CREATE TABLE IF NOT EXISTS catalog_snapshots(id INTEGER PRIMARY KEY CHECK(id=1),hash TEXT NOT NULL,catalog BLOB NOT NULL,notified INTEGER NOT NULL,updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS queue_jobs(id INTEGER PRIMARY KEY AUTOINCREMENT,command TEXT NOT NULL,params_json TEXT NOT NULL,idempotency_key TEXT UNIQUE,max_attempts INTEGER NOT NULL,attempts INTEGER NOT NULL DEFAULT 0,available_at TEXT NOT NULL,lease_until TEXT,status TEXT NOT NULL DEFAULT 'pending',result TEXT);
CREATE INDEX IF NOT EXISTS queue_ready ON queue_jobs(status,available_at);
CREATE TABLE IF NOT EXISTS outbox_replies(id INTEGER PRIMARY KEY AUTOINCREMENT,message_id TEXT NOT NULL UNIQUE,recipient TEXT NOT NULL,payload BLOB NOT NULL,max_attempts INTEGER NOT NULL,attempts INTEGER NOT NULL DEFAULT 0,available_at TEXT NOT NULL,lease_until TEXT,status TEXT NOT NULL DEFAULT 'pending',last_error TEXT,created_at TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS outbox_ready ON outbox_replies(status,available_at);
CREATE TABLE IF NOT EXISTS runtime_state(key TEXT PRIMARY KEY,value TEXT NOT NULL,updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS runtime_events(id INTEGER PRIMARY KEY AUTOINCREMENT,at TEXT NOT NULL,severity TEXT NOT NULL,phase TEXT NOT NULL,message_id TEXT NOT NULL DEFAULT '',command TEXT NOT NULL DEFAULT '',handler TEXT NOT NULL DEFAULT '',error_kind TEXT NOT NULL DEFAULT '',summary TEXT NOT NULL DEFAULT '');
CREATE INDEX IF NOT EXISTS runtime_events_recent ON runtime_events(id DESC);`)
	if err != nil {
		return err
	}
	for _, col := range []struct {
		name string
		def  string
	}{
		{name: "command", def: "TEXT NOT NULL DEFAULT ''"},
		{name: "error_kind", def: "TEXT NOT NULL DEFAULT ''"},
		{name: "error_summary", def: "TEXT NOT NULL DEFAULT ''"},
		{name: "updated_at", def: "TEXT NOT NULL DEFAULT ''"},
	} {
		if err := s.addColumn("processed_messages", col.name, col.def); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) addColumn(table, column, definition string) error {
	_, err := s.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition))
	if err != nil && strings.Contains(err.Error(), "duplicate column name") {
		return nil
	}
	return err
}
func (s *Store) RecordExecutionAndReply(ctx context.Context, e Execution, params map[string]any, recipient string, payload []byte, maxAttempts int) (int64, error) {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	b, err := json.Marshal(params)
	if err != nil {
		return 0, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO executions(message_id,command,handler,params_json,status,summary,error,started_at,duration_ms) VALUES(?,?,?,?,?,?,?,?,?)`, e.MessageID, e.Command, e.Handler, string(b), e.Status, e.Summary, e.Error, dbTime(e.StartedAt), e.Duration.Milliseconds())
	if err != nil {
		return 0, err
	}
	now := dbTime(time.Now())
	res, err := tx.ExecContext(ctx, `INSERT INTO outbox_replies(message_id,recipient,payload,max_attempts,available_at,created_at) VALUES(?,?,?,?,?,?) ON CONFLICT(message_id) DO NOTHING`, e.MessageID, recipient, payload, maxAttempts, now, now)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if id == 0 {
		if err = tx.QueryRowContext(ctx, `SELECT id FROM outbox_replies WHERE message_id=?`, e.MessageID).Scan(&id); err != nil {
			return 0, err
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE processed_messages SET status=?,reply_pending=1,command=?,error_kind='',error_summary='',updated_at=? WHERE id=?`, MessageReplyPending, e.Command, now, e.MessageID); err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}
func (s *Store) ClaimReply(ctx context.Context, now time.Time, lease time.Duration) (*Reply, error) {
	return s.claimReply(ctx, 0, now, lease)
}
func (s *Store) ClaimReplyID(ctx context.Context, id int64, now time.Time, lease time.Duration) (*Reply, error) {
	return s.claimReply(ctx, id, now, lease)
}
func (s *Store) claimReply(ctx context.Context, id int64, now time.Time, lease time.Duration) (*Reply, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := s.expireFinalAttemptReplies(ctx, tx, now); err != nil {
		return nil, err
	}
	var r Reply
	var at string
	query := `SELECT id,message_id,recipient,payload,attempts,max_attempts,available_at FROM outbox_replies WHERE (status='pending' OR (status='running' AND lease_until<?)) AND available_at<=? AND attempts<max_attempts`
	args := []any{dbTime(now), dbTime(now)}
	if id > 0 {
		query += ` AND id=?`
		args = append(args, id)
	}
	query += ` ORDER BY id LIMIT 1`
	err = tx.QueryRowContext(ctx, query, args...).Scan(&r.ID, &r.MessageID, &r.Recipient, &r.Payload, &r.Attempts, &r.MaxAttempts, &at)
	if errors.Is(err, sql.ErrNoRows) {
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	res, err := tx.ExecContext(ctx, `UPDATE outbox_replies SET status='running',attempts=attempts+1,lease_until=? WHERE id=?`, dbTime(now.Add(lease)), r.ID)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return nil, fmt.Errorf("reply claim lost")
	}
	r.Attempts++
	r.AvailableAt, _ = parseDBTime(at)
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) expireFinalAttemptReplies(ctx context.Context, tx *sql.Tx, now time.Time) error {
	rows, err := tx.QueryContext(ctx, `SELECT id,message_id FROM outbox_replies WHERE status='running' AND lease_until<? AND attempts>=max_attempts`, dbTime(now))
	if err != nil {
		return err
	}
	defer rows.Close()
	type expiredReply struct {
		id        int64
		messageID string
	}
	var expired []expiredReply
	for rows.Next() {
		var r expiredReply
		if err := rows.Scan(&r.id, &r.messageID); err != nil {
			return err
		}
		expired = append(expired, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	reason := safeExpiredLeaseReason("reply")
	for _, r := range expired {
		if _, err := tx.ExecContext(ctx, `UPDATE outbox_replies SET status='dead',last_error=?,lease_until=NULL WHERE id=?`, reason, r.id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO processed_messages(id,sender,status,reply_pending,command,error_kind,error_summary,updated_at)
VALUES(?,?,?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET status=excluded.status,reply_pending=excluded.reply_pending,error_kind=excluded.error_kind,error_summary=excluded.error_summary,updated_at=excluded.updated_at`,
			r.messageID, "", MessageDead, 0, "", "reply_delivery", reason, dbTime(now)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CompleteReply(ctx context.Context, r *Reply) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE outbox_replies SET status='done',lease_until=NULL,last_error=NULL WHERE id=?`, r.ID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE processed_messages SET status=?,reply_pending=0,updated_at=? WHERE id=?`, MessageDone, dbTime(time.Now()), r.MessageID); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) FailReply(ctx context.Context, r *Reply, reason string, backoff time.Duration) error {
	reason = safeReplyFailureReason(reason)
	status := "pending"
	if r.Attempts >= r.MaxAttempts {
		status = "dead"
	}
	now := dbTime(time.Now())
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE outbox_replies SET status=?,last_error=?,available_at=?,lease_until=NULL WHERE id=?`, status, reason, dbTime(time.Now().Add(backoff)), r.ID); err != nil {
		return err
	}
	if status == "dead" {
		if _, err = tx.ExecContext(ctx, `INSERT INTO processed_messages(id,sender,status,reply_pending,command,error_kind,error_summary,updated_at)
VALUES(?,?,?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET status=excluded.status,reply_pending=excluded.reply_pending,error_kind=excluded.error_kind,error_summary=excluded.error_summary,updated_at=excluded.updated_at`,
			r.MessageID, "", MessageDead, 0, "", "reply_delivery", reason, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Store) ReplyCounts(ctx context.Context) (pending, dead int, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT count(*) FILTER (WHERE status IN ('pending','running')),count(*) FILTER (WHERE status='dead') FROM outbox_replies`).Scan(&pending, &dead)
	return
}
func (s *Store) ReplayReply(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var messageID string
	err = tx.QueryRowContext(ctx, `SELECT message_id FROM outbox_replies WHERE id=? AND status='dead'`, id).Scan(&messageID)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("dead reply %d not found", id)
	}
	if err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `UPDATE outbox_replies SET status='pending',attempts=0,available_at=?,lease_until=NULL,last_error=NULL WHERE id=? AND status='dead'`, dbTime(time.Now()), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return fmt.Errorf("dead reply %d not found", id)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE processed_messages SET status=?,reply_pending=1,updated_at=? WHERE id=?`, MessageReplyPending, dbTime(time.Now()), messageID); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) ClaimMessage(ctx context.Context, id, sender string) (bool, error) {
	r, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO processed_messages(id,sender,status,updated_at) VALUES(?,?,?,?)`, id, sender, MessageClaimed, dbTime(time.Now()))
	if err != nil {
		return false, err
	}
	n, err := r.RowsAffected()
	return n == 1, err
}
func (s *Store) MarkMessage(ctx context.Context, id, status string, replyPending bool) error {
	_, err := s.db.ExecContext(ctx, `UPDATE processed_messages SET status=?,reply_pending=?,updated_at=? WHERE id=?`, status, boolInt(replyPending), dbTime(time.Now()), id)
	return err
}
func (s *Store) RecordMessageFailure(ctx context.Context, u MessageUpdate) error {
	if u.ID == "" {
		u.ID = "generated:" + dbTime(time.Now()) + ":" + u.State.String()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO processed_messages(id,sender,status,reply_pending,command,error_kind,error_summary,updated_at)
VALUES(?,?,?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET sender=CASE WHEN excluded.sender='' THEN processed_messages.sender ELSE excluded.sender END,status=excluded.status,reply_pending=excluded.reply_pending,command=CASE WHEN excluded.command='' THEN processed_messages.command ELSE excluded.command END,error_kind=excluded.error_kind,error_summary=excluded.error_summary,updated_at=excluded.updated_at`,
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
func (s *Store) AddExecution(ctx context.Context, e Execution, params map[string]any) (int64, error) {
	b, err := json.Marshal(params)
	if err != nil {
		return 0, err
	}
	r, err := s.db.ExecContext(ctx, `INSERT INTO executions(message_id,command,handler,params_json,status,summary,error,started_at,duration_ms) VALUES(?,?,?,?,?,?,?,?,?)`, e.MessageID, e.Command, e.Handler, string(b), e.Status, e.Summary, e.Error, dbTime(e.StartedAt), e.Duration.Milliseconds())
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}
func (s *Store) RecentExecution(ctx context.Context) (Execution, error) {
	var e Execution
	var started string
	var ms int64
	err := s.db.QueryRowContext(ctx, `SELECT id,COALESCE(message_id,''),command,handler,status,COALESCE(summary,''),COALESCE(error,''),started_at,duration_ms FROM executions ORDER BY id DESC LIMIT 1`).Scan(&e.ID, &e.MessageID, &e.Command, &e.Handler, &e.Status, &e.Summary, &e.Error, &started, &ms)
	if err != nil {
		return e, err
	}
	e.StartedAt, _ = parseDBTime(started)
	e.Duration = time.Duration(ms) * time.Millisecond
	return e, nil
}
func (s *Store) SaveCatalog(ctx context.Context, hash string, catalog []byte, notified bool) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO catalog_snapshots(id,hash,catalog,notified,updated_at) VALUES(1,?,?,?,?) ON CONFLICT(id) DO UPDATE SET hash=excluded.hash,catalog=excluded.catalog,notified=excluded.notified,updated_at=excluded.updated_at`, hash, catalog, notified, dbTime(time.Now()))
	return err
}
func (s *Store) Catalog(ctx context.Context) (string, []byte, bool, error) {
	var h string
	var b []byte
	var n bool
	err := s.db.QueryRowContext(ctx, `SELECT hash,catalog,notified FROM catalog_snapshots WHERE id=1`).Scan(&h, &b, &n)
	return h, b, n, err
}
func (s *Store) SetState(ctx context.Context, k, v string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO runtime_state(key,value,updated_at) VALUES(?,?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value,updated_at=excluded.updated_at`, k, v, dbTime(time.Now()))
	return err
}
func (s *Store) State(ctx context.Context, k string) (string, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM runtime_state WHERE key=?`, k).Scan(&v)
	return v, err
}
func (s *Store) AddEvent(ctx context.Context, e RuntimeEvent) error {
	if e.At.IsZero() {
		e.At = time.Now()
	}
	if e.Severity == "" {
		e.Severity = "info"
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO runtime_events(at,severity,phase,message_id,command,handler,error_kind,summary) VALUES(?,?,?,?,?,?,?,?)`,
		dbTime(e.At), e.Severity, e.Phase, e.MessageID, e.Command, e.Handler, e.ErrorKind, e.Summary)
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
func (s *Store) Health(ctx context.Context) (HealthSummary, error) {
	var h HealthSummary
	err := s.db.QueryRowContext(ctx, `SELECT
		(SELECT count(*) FROM queue_jobs WHERE status='pending'),
		(SELECT count(*) FROM queue_jobs WHERE status='running'),
		(SELECT count(*) FROM queue_jobs WHERE status='dead'),
		(SELECT count(*) FROM outbox_replies WHERE status='pending'),
		(SELECT count(*) FROM outbox_replies WHERE status='running'),
		(SELECT count(*) FROM outbox_replies WHERE status='dead'),
		(SELECT count(*) FROM processed_messages WHERE status='executing')`).
		Scan(&h.QueuePending, &h.QueueRunning, &h.QueueDead, &h.ReplyPending, &h.ReplyRunning, &h.ReplyDead, &h.StaleExecuting)
	if err != nil {
		return h, err
	}
	h.LatestFailures, err = s.recentErrorEvents(ctx, 5)
	return h, err
}

func (s *Store) recentErrorEvents(ctx context.Context, limit int) ([]RuntimeEvent, error) {
	if limit < 1 {
		limit = 1
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,at,severity,phase,message_id,command,handler,error_kind,summary FROM runtime_events WHERE severity='error' ORDER BY id DESC LIMIT ?`, limit)
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

func (s *Store) Enqueue(ctx context.Context, cmd string, params map[string]any, key string, max int, at time.Time) (int64, error) {
	if max < 1 {
		max = 1
	}
	b, err := json.Marshal(params)
	if err != nil {
		return 0, err
	}
	r, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO queue_jobs(command,params_json,idempotency_key,max_attempts,available_at) VALUES(?,?,?,?,?)`, cmd, string(b), key, max, dbTime(at))
	if err != nil {
		return 0, err
	}
	id, err := r.LastInsertId()
	if err == nil && id == 0 {
		err = s.db.QueryRowContext(ctx, `SELECT id FROM queue_jobs WHERE idempotency_key=?`, key).Scan(&id)
	}
	return id, err
}
func (s *Store) ClaimJob(ctx context.Context, now time.Time, lease time.Duration) (*Job, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE queue_jobs SET status='dead',result=?,lease_until=NULL WHERE status='running' AND lease_until<? AND attempts>=max_attempts`, safeExpiredLeaseReason("queue"), dbTime(now)); err != nil {
		return nil, err
	}
	var j Job
	var raw, at string
	err = tx.QueryRowContext(ctx, `SELECT id,command,params_json,attempts,max_attempts,available_at FROM queue_jobs WHERE (status='pending' OR (status='running' AND lease_until<?)) AND available_at<=? AND attempts<max_attempts ORDER BY id LIMIT 1`, dbTime(now), dbTime(now)).Scan(&j.ID, &j.Command, &raw, &j.Attempts, &j.MaxAttempts, &at)
	if errors.Is(err, sql.ErrNoRows) {
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r, err := tx.ExecContext(ctx, `UPDATE queue_jobs SET status='running',attempts=attempts+1,lease_until=? WHERE id=?`, dbTime(now.Add(lease)), j.ID)
	if err != nil {
		return nil, err
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return nil, fmt.Errorf("queue claim lost")
	}
	j.Attempts++
	j.AvailableAt, _ = parseDBTime(at)
	if err = json.Unmarshal([]byte(raw), &j.Params); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &j, nil
}
func (s *Store) CompleteJob(ctx context.Context, id int64, result string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE queue_jobs SET status='done',result=?,lease_until=NULL WHERE id=?`, result, id)
	return err
}
func (s *Store) FailJob(ctx context.Context, j *Job, reason string, backoff time.Duration) error {
	reason = safeQueueFailureReason(reason)
	status := "pending"
	if j.Attempts >= j.MaxAttempts {
		status = "dead"
	}
	_, err := s.db.ExecContext(ctx, `UPDATE queue_jobs SET status=?,result=?,available_at=?,lease_until=NULL WHERE id=?`, status, reason, dbTime(time.Now().Add(backoff)), j.ID)
	return err
}
func (s *Store) RejectJob(ctx context.Context, j *Job, kind string) error {
	switch kind {
	case "unknown_command", "invalid_parameters", "policy":
	default:
		kind = "policy"
	}
	res, err := s.db.ExecContext(ctx, `UPDATE queue_jobs SET status='dead',result=?,lease_until=NULL WHERE id=? AND status='running'`, kind, j.ID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("running queue job %d not found", j.ID)
	}
	return nil
}
func (s *Store) DeadCounts(ctx context.Context) (queueDead, replyDead int, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM queue_jobs WHERE status='dead'),(SELECT count(*) FROM outbox_replies WHERE status='dead')`).Scan(&queueDead, &replyDead)
	return
}
func (s *Store) ReplayJob(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `UPDATE queue_jobs SET status='pending',attempts=0,available_at=?,lease_until=NULL,result=NULL WHERE id=? AND status='dead'`, dbTime(time.Now()), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return fmt.Errorf("dead queue job %d not found", id)
	}
	return nil
}
func (s *Store) LatestFailure(ctx context.Context) (string, error) {
	var reason string
	err := s.db.QueryRowContext(ctx, `SELECT last_error FROM outbox_replies WHERE status='dead' AND last_error IS NOT NULL ORDER BY id DESC LIMIT 1`).Scan(&reason)
	if err == nil {
		return "reply: " + reason, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	err = s.db.QueryRowContext(ctx, `SELECT result FROM queue_jobs WHERE status='dead' AND result IS NOT NULL ORDER BY id DESC LIMIT 1`).Scan(&reason)
	if err != nil {
		return "", err
	}
	return "queue: " + reason, nil
}
func (s *Store) QueueDepth(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM queue_jobs WHERE status IN ('pending','running')`).Scan(&n)
	return n, err
}

func (s *Store) ConsoleExecutions(ctx context.Context, beforeID int64, limit int, status, commandName string) ([]ExecutionRecord, error) {
	query := `SELECT e.id,e.message_id,e.command,e.handler,e.status,COALESCE(e.summary,''),COALESCE(e.error,''),e.started_at,e.duration_ms,COALESCE(m.sender,'')
FROM executions e LEFT JOIN processed_messages m ON m.id=e.message_id WHERE 1=1`
	args := make([]any, 0, 4)
	if beforeID > 0 {
		query += ` AND e.id<?`
		args = append(args, beforeID)
	}
	if status != "" {
		query += ` AND e.status=?`
		args = append(args, status)
	}
	if commandName != "" {
		query += ` AND e.command=?`
		args = append(args, commandName)
	}
	query += ` ORDER BY e.id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ExecutionRecord
	for rows.Next() {
		var item ExecutionRecord
		var started string
		var durationMS int64
		if err := rows.Scan(&item.ID, &item.MessageID, &item.Command, &item.Handler, &item.Status, &item.Summary, &item.Error, &started, &durationMS, &item.Sender); err != nil {
			return nil, err
		}
		item.StartedAt, _ = parseDBTime(started)
		item.Duration = time.Duration(durationMS) * time.Millisecond
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ConsoleJobs(ctx context.Context, beforeID int64, limit int, status string) ([]JobRecord, error) {
	query := `SELECT id,command,status,attempts,max_attempts,available_at FROM queue_jobs WHERE 1=1`
	args := make([]any, 0, 3)
	if beforeID > 0 {
		query += ` AND id<?`
		args = append(args, beforeID)
	}
	if status != "" {
		query += ` AND status=?`
		args = append(args, status)
	}
	query += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []JobRecord
	for rows.Next() {
		var item JobRecord
		var available string
		if err := rows.Scan(&item.ID, &item.Command, &item.Status, &item.Attempts, &item.MaxAttempts, &available); err != nil {
			return nil, err
		}
		item.AvailableAt, _ = parseDBTime(available)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ConsoleReplies(ctx context.Context, beforeID int64, limit int, status string) ([]ReplyRecord, error) {
	query := `SELECT id,message_id,recipient,status,attempts,max_attempts,available_at,created_at,COALESCE(last_error,'') FROM outbox_replies WHERE 1=1`
	args := make([]any, 0, 3)
	if beforeID > 0 {
		query += ` AND id<?`
		args = append(args, beforeID)
	}
	if status != "" {
		query += ` AND status=?`
		args = append(args, status)
	}
	query += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ReplyRecord
	for rows.Next() {
		var item ReplyRecord
		var available, created string
		if err := rows.Scan(&item.ID, &item.MessageID, &item.Recipient, &item.Status, &item.Attempts, &item.MaxAttempts, &available, &created, &item.LastError); err != nil {
			return nil, err
		}
		item.AvailableAt, _ = parseDBTime(available)
		item.CreatedAt, _ = parseDBTime(created)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ConsoleEvents(ctx context.Context, beforeID int64, limit int, severity string) ([]RuntimeEvent, error) {
	query := `SELECT id,at,severity,phase,message_id,command,handler,error_kind,summary FROM runtime_events WHERE 1=1`
	args := make([]any, 0, 3)
	if beforeID > 0 {
		query += ` AND id<?`
		args = append(args, beforeID)
	}
	if severity != "" {
		query += ` AND severity=?`
		args = append(args, severity)
	}
	query += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RuntimeEvent
	for rows.Next() {
		var item RuntimeEvent
		var at string
		if err := rows.Scan(&item.ID, &at, &item.Severity, &item.Phase, &item.MessageID, &item.Command, &item.Handler, &item.ErrorKind, &item.Summary); err != nil {
			return nil, err
		}
		item.At, _ = parseDBTime(at)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ConsoleCountsSince(ctx context.Context, since time.Time) (ConsoleCounts, []time.Duration, error) {
	var counts ConsoleCounts
	err := s.db.QueryRowContext(ctx, `SELECT count(*),count(*) FILTER (WHERE status='success'),count(DISTINCT handler) FROM executions WHERE started_at>=?`, dbTime(since)).Scan(&counts.Executions, &counts.Success, &counts.ActiveHandlers)
	if err != nil {
		return counts, nil, err
	}
	err = s.db.QueryRowContext(ctx, `SELECT count(*) FILTER (WHERE status='pending'),count(*) FILTER (WHERE status='running'),count(*) FILTER (WHERE status='dead') FROM queue_jobs`).Scan(&counts.QueuePending, &counts.QueueRunning, &counts.QueueDead)
	if err != nil {
		return counts, nil, err
	}
	err = s.db.QueryRowContext(ctx, `SELECT count(*) FILTER (WHERE status='pending'),count(*) FILTER (WHERE status='running'),count(*) FILTER (WHERE status='dead') FROM outbox_replies`).Scan(&counts.ReplyPending, &counts.ReplyRunning, &counts.ReplyDead)
	if err != nil {
		return counts, nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT duration_ms FROM executions WHERE started_at>=? ORDER BY duration_ms`, dbTime(since))
	if err != nil {
		return counts, nil, err
	}
	defer rows.Close()
	var durations []time.Duration
	for rows.Next() {
		var ms int64
		if err := rows.Scan(&ms); err != nil {
			return counts, nil, err
		}
		durations = append(durations, time.Duration(ms)*time.Millisecond)
	}
	return counts, durations, rows.Err()
}

// ConsoleSeriesSince returns execution counts grouped into `buckets` equal-width
// time buckets spanning [since, now]. Each bucket carries the total and the
// successful execution count so the console can render an accurate trend.
func (s *Store) ConsoleSeriesSince(ctx context.Context, since time.Time, buckets int) ([]SeriesPoint, error) {
	if buckets < 1 {
		buckets = 1
	}
	now := time.Now()
	if !since.Before(now) {
		since = now.Add(-time.Minute)
	}
	width := now.Sub(since) / time.Duration(buckets)
	if width <= 0 {
		width = time.Nanosecond
	}
	points := make([]SeriesPoint, buckets)
	for i := range points {
		points[i].BucketStart = since.Add(time.Duration(i) * width)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT started_at,status FROM executions WHERE started_at>=? ORDER BY started_at`, dbTime(since))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var startedRaw string
		var status string
		if err := rows.Scan(&startedRaw, &status); err != nil {
			return nil, err
		}
		startedAt, err := parseDBTime(startedRaw)
		if err != nil {
			return nil, err
		}
		idx := int(startedAt.Sub(since) / width)
		if idx < 0 {
			idx = 0
		}
		if idx >= buckets {
			idx = buckets - 1
		}
		points[idx].Count++
		if status == "success" {
			points[idx].Success++
		}
	}
	return points, rows.Err()
}
