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
CREATE TABLE IF NOT EXISTS runtime_state(key TEXT PRIMARY KEY,value TEXT NOT NULL,updated_at TEXT NOT NULL);`)
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
	res, err := s.db.ExecContext(ctx, `UPDATE outbox_replies SET status='pending',attempts=0,available_at=?,lease_until=NULL,last_error=NULL WHERE id=? AND status='dead'`, dbTime(time.Now()), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return fmt.Errorf("dead reply %d not found", id)
	}
	return nil
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
	var j Job
	var raw, at string
	err = tx.QueryRowContext(ctx, `SELECT id,command,params_json,attempts,max_attempts,available_at FROM queue_jobs WHERE (status='pending' OR (status='running' AND lease_until<?)) AND available_at<=? AND attempts<max_attempts ORDER BY id LIMIT 1`, dbTime(now), dbTime(now)).Scan(&j.ID, &j.Command, &raw, &j.Attempts, &j.MaxAttempts, &at)
	if errors.Is(err, sql.ErrNoRows) {
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
	status := "pending"
	if j.Attempts >= j.MaxAttempts {
		status = "dead"
	}
	_, err := s.db.ExecContext(ctx, `UPDATE queue_jobs SET status=?,result=?,available_at=?,lease_until=NULL WHERE id=?`, status, reason, dbTime(time.Now().Add(backoff)), j.ID)
	return err
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
