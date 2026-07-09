package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/becomeopc/opc-mailrelay/internal/command"
	"github.com/becomeopc/opc-mailrelay/internal/store"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type failingExec struct{}

func (f *failingExec) Execute(context.Context, command.Request) (command.Result, error) {
	return command.Result{}, fmt.Errorf("job failed")
}

type rawFailingExec struct{}

func (f *rawFailingExec) Execute(context.Context, command.Request) (command.Result, error) {
	return command.Result{}, fmt.Errorf("webhook 500 for vip@example.com token=topsecret")
}

type typedFailingExec struct {
	kind    string
	message string
}

func (f typedFailingExec) Execute(context.Context, command.Request) (command.Result, error) {
	return command.Result{}, &command.Error{Kind: f.kind, Message: f.message}
}

type catalogExec struct {
	commands map[string]command.Command
}

func (e catalogExec) Execute(context.Context, command.Request) (command.Result, error) {
	return command.Result{Status: "success", Summary: "ok"}, nil
}

func (e catalogExec) Command(name string) (command.Command, bool) {
	c, ok := e.commands[name]
	return c, ok
}

func TestQueueAndWorker(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	h := NewQueue(s)
	_, err = h.Execute(context.Background(), command.Context{Command: command.Command{Name: "later", Config: map[string]any{"command": "deploy", "max_attempts": 2}}, Request: command.Request{MessageID: "m1", Params: map[string]any{"env": "prod"}}})
	if err != nil {
		t.Fatal(err)
	}
	e := &execCapture{}
	worked, err := RunOneJob(context.Background(), s, e, time.Second)
	if err != nil || !worked || len(e.names) != 1 || e.names[0] != "deploy" {
		t.Fatalf("%v %#v %v", worked, e.names, err)
	}
}

func TestQueueRedactsSensitiveParamsAtHandlerBoundary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "q-redact.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	h := NewQueue(s)
	cmd := command.Command{
		Name: "later",
		Config: map[string]any{
			"command": "deploy",
		},
		Parameters: map[string]command.Parameter{
			"env":    {Type: "string"},
			"secret": {Type: "string", Sensitive: true},
		},
	}
	_, err = h.Execute(context.Background(), command.Context{
		Command: cmd,
		Request: command.Request{MessageID: "m-redact", Params: map[string]any{"env": "prod", "secret": "raw-secret"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var raw string
	if err := db.QueryRow(`SELECT params_json FROM queue_jobs WHERE idempotency_key='m-redact:later'`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "raw-secret") {
		t.Fatalf("raw secret leaked into queue params: %s", raw)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatal(err)
	}
	if got["secret"] != "[REDACTED]" || got["env"] != "prod" {
		t.Fatalf("unexpected queue params: %s", raw)
	}
}

func TestQueueRedactsParamsUsingTargetCommandMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "q-target-redact.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	h := NewQueue(s)
	wrapper := command.Command{
		Name:       "later",
		Handler:    "queue",
		Config:     map[string]any{"command": "deploy"},
		Parameters: map[string]command.Parameter{"env": {Type: "string"}, "secret": {Type: "string"}},
	}
	target := command.Command{
		Name:    "deploy",
		Handler: "capture",
		Parameters: map[string]command.Parameter{
			"env":    {Type: "string"},
			"secret": {Type: "string", Sensitive: true},
		},
	}

	_, err = h.Execute(context.Background(), command.Context{
		Command: wrapper,
		Request: command.Request{
			MessageID: "m-target-redact",
			Name:      "later",
			Params:    map[string]any{"env": "prod", "secret": "target-secret"},
		},
		Execute: catalogExec{commands: map[string]command.Command{"deploy": target}},
	})
	if err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var raw string
	if err := db.QueryRow(`SELECT params_json FROM queue_jobs WHERE idempotency_key='m-target-redact:later'`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "target-secret") {
		t.Fatalf("raw target secret leaked into queue params: %s", raw)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatal(err)
	}
	if got["secret"] != "[REDACTED]" || got["env"] != "prod" {
		t.Fatalf("unexpected queue params: %s", raw)
	}
}

func TestQueueRedactsParamsUsingWrapperAndTargetMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "q-union-redact.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	h := NewQueue(s)
	wrapper := command.Command{
		Name:    "later",
		Handler: "queue",
		Config:  map[string]any{"command": "deploy"},
		Parameters: map[string]command.Parameter{
			"env":   {Type: "string"},
			"token": {Type: "string", Sensitive: true},
		},
	}
	target := command.Command{
		Name:    "deploy",
		Handler: "capture",
		Parameters: map[string]command.Parameter{
			"env":   {Type: "string"},
			"token": {Type: "string"},
		},
	}

	_, err = h.Execute(context.Background(), command.Context{
		Command: wrapper,
		Request: command.Request{
			MessageID: "m-wrapper-redact",
			Name:      "later",
			Params:    map[string]any{"env": "prod", "token": "wrapper-secret"},
		},
		Execute: catalogExec{commands: map[string]command.Command{"deploy": target}},
	})
	if err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var raw string
	if err := db.QueryRow(`SELECT params_json FROM queue_jobs WHERE idempotency_key='m-wrapper-redact:later'`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "wrapper-secret") {
		t.Fatalf("raw wrapper secret leaked into queue params: %s", raw)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatal(err)
	}
	if got["token"] != "[REDACTED]" || got["env"] != "prod" {
		t.Fatalf("unexpected queue params: %s", raw)
	}
}

func TestQueueUsesRuntimeDefaultAttemptsAndBackoff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "q-policy.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	h := NewQueue(s)
	h.SetRetryPolicy(7, 10*time.Second, 15*time.Second)
	_, err = h.Execute(context.Background(), command.Context{
		Command: command.Command{Name: "later", Config: map[string]any{"command": "deploy"}},
		Request: command.Request{MessageID: "m-policy", Params: map[string]any{"env": "prod"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var maxAttempts int
	if err := db.QueryRow(`SELECT max_attempts FROM queue_jobs WHERE idempotency_key='m-policy:later'`).Scan(&maxAttempts); err != nil {
		t.Fatal(err)
	}
	if maxAttempts != 7 {
		t.Fatalf("max_attempts=%d, want runtime default 7", maxAttempts)
	}

	e := &failingExec{}
	before := time.Now()
	worked, err := RunOneJobWithPolicy(context.Background(), s, e, time.Second, 10*time.Second, 15*time.Second)
	if err != nil || !worked {
		t.Fatalf("worked=%v err=%v", worked, err)
	}
	var availableRaw string
	if err := db.QueryRow(`SELECT available_at FROM queue_jobs WHERE idempotency_key='m-policy:later'`).Scan(&availableRaw); err != nil {
		t.Fatal(err)
	}
	availableAt, err := time.Parse("2006-01-02T15:04:05.000000000Z07:00", availableRaw)
	if err != nil {
		t.Fatal(err)
	}
	delay := availableAt.Sub(before)
	if delay < 9*time.Second || delay > 16*time.Second {
		t.Fatalf("queue backoff delay=%s, want about 10s capped by policy", delay)
	}
}

func TestQueueCommandMaxAttemptsOverridesRuntimeDefault(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "q-override.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	h := NewQueue(s)
	h.SetRetryPolicy(7, time.Second, time.Minute)
	_, err = h.Execute(context.Background(), command.Context{
		Command: command.Command{Name: "later", Config: map[string]any{"command": "deploy", "max_attempts": 2}},
		Request: command.Request{MessageID: "m-override"},
	})
	if err != nil {
		t.Fatal(err)
	}
	j, err := s.ClaimJob(context.Background(), time.Now(), time.Second)
	if err != nil || j == nil {
		t.Fatalf("job=%#v err=%v", j, err)
	}
	if j.MaxAttempts != 2 {
		t.Fatalf("max_attempts=%d, want command override 2", j.MaxAttempts)
	}
}

func TestRunOneJobStoresSafeFailureSummary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "q-safe-failure.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	h := NewQueue(s)
	_, err = h.Execute(context.Background(), command.Context{
		Command: command.Command{Name: "later", Config: map[string]any{"command": "deploy", "max_attempts": 1}},
		Request: command.Request{MessageID: "m-safe", Params: map[string]any{"env": "prod"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	worked, err := RunOneJob(context.Background(), s, &rawFailingExec{}, time.Second)
	if err != nil || !worked {
		t.Fatalf("worked=%v err=%v", worked, err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var result string
	if err := db.QueryRow(`SELECT result FROM queue_jobs WHERE idempotency_key='m-safe:later'`).Scan(&result); err != nil {
		t.Fatal(err)
	}
	if result != "dependency" {
		t.Fatalf("queue result=%q, want sanitized dependency", result)
	}
	if strings.Contains(result, "vip@example.com") || strings.Contains(result, "token=topsecret") {
		t.Fatalf("raw worker error leaked into queue result: %q", result)
	}
}

func TestQueueTerminalFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "q-terminal.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Enqueue(context.Background(), "missing", nil, "terminal", 5, time.Now()); err != nil {
		t.Fatal(err)
	}
	worked, err := RunOneJobWithPolicy(context.Background(), s, typedFailingExec{kind: "unknown_command", message: "token=topsecret"}, time.Minute, time.Second, time.Minute)
	if err != nil || !worked {
		t.Fatalf("worked=%v err=%v", worked, err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var status, result string
	var attempts int
	if err := db.QueryRow(`SELECT status,result,attempts FROM queue_jobs WHERE idempotency_key='terminal'`).Scan(&status, &result, &attempts); err != nil {
		t.Fatal(err)
	}
	if status != "dead" || result != "unknown_command" || attempts != 1 {
		t.Fatalf("status=%q result=%q attempts=%d", status, result, attempts)
	}
}

func TestQueueRetryableFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "q-retryable.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Enqueue(context.Background(), "deploy", nil, "retryable", 5, time.Now()); err != nil {
		t.Fatal(err)
	}
	worked, err := RunOneJobWithPolicy(context.Background(), s, typedFailingExec{kind: "dependency", message: "upstream unavailable"}, time.Minute, time.Second, time.Minute)
	if err != nil || !worked {
		t.Fatalf("worked=%v err=%v", worked, err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var status, result string
	if err := db.QueryRow(`SELECT status,result FROM queue_jobs WHERE idempotency_key='retryable'`).Scan(&status, &result); err != nil {
		t.Fatal(err)
	}
	if status != "pending" || result != "dependency" {
		t.Fatalf("status=%q result=%q", status, result)
	}
}

func TestQueueFailureEventIsSafe(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "q-event.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Enqueue(context.Background(), "deploy", nil, "event", 2, time.Now()); err != nil {
		t.Fatal(err)
	}
	worked, err := RunOneJobWithPolicy(context.Background(), s, typedFailingExec{kind: "dependency", message: "vip@example.com token=topsecret"}, time.Minute, time.Second, time.Minute)
	if err != nil || !worked {
		t.Fatalf("worked=%v err=%v", worked, err)
	}
	events, err := s.RecentEvents(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events=%#v", events)
	}
	event := events[0]
	if event.Phase != "queue" || event.Handler != "queue" || event.ErrorKind != "dependency" || !strings.HasPrefix(event.Summary, "queue job failed:") {
		t.Fatalf("event=%#v", event)
	}
	if !strings.Contains(event.Summary, "[email]") || !strings.Contains(event.Summary, "[REDACTED]") {
		t.Fatalf("event should include sanitized detail=%#v", event)
	}
	if strings.Contains(event.Summary, "vip@example.com") || strings.Contains(event.Summary, "topsecret") {
		t.Fatalf("unsafe event=%#v", event)
	}
}
