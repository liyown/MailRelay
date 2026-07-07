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

func TestQueueRedactsSensitiveParamsButWorkerReceivesRedactedCopy(t *testing.T) {
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
