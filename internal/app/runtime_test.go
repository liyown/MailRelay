package app

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"github.com/becomeopc/opc-mailrelay/internal/command"
	"github.com/becomeopc/opc-mailrelay/internal/handler"
	"github.com/becomeopc/opc-mailrelay/internal/mailbox"
	"github.com/becomeopc/opc-mailrelay/internal/store"
	webconsole "github.com/becomeopc/opc-mailrelay/internal/web"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type workflowLifecycleExec struct{ names []string }

func (e *workflowLifecycleExec) Execute(_ context.Context, req command.Request) (command.Result, error) {
	e.names = append(e.names, req.Name)
	if req.Name == "deploy" {
		return command.Result{}, &command.Error{Kind: "dependency", Message: "target unavailable"}
	}
	return command.Result{Status: "success", Summary: "ok"}, nil
}

type runCancelReceiver struct{ cancel context.CancelFunc }

func (r *runCancelReceiver) Fetch(context.Context, int) ([]mailbox.RawMessage, error) {
	return nil, nil
}
func (r *runCancelReceiver) MarkSeen(context.Context, uint32) error { return nil }
func (r *runCancelReceiver) Idle(context.Context) error {
	r.cancel()
	return context.Canceled
}

type failingReceiver struct{ err error }

func (r *failingReceiver) Fetch(context.Context, int) ([]mailbox.RawMessage, error) {
	return nil, r.err
}
func (r *failingReceiver) MarkSeen(context.Context, uint32) error { return nil }
func (r *failingReceiver) Idle(context.Context) error             { return nil }

type cancelOnFetchReceiver struct {
	err    error
	cancel context.CancelFunc
}

type blockingReceiver struct{}

func (*blockingReceiver) Fetch(context.Context, int) ([]mailbox.RawMessage, error) { return nil, nil }
func (*blockingReceiver) MarkSeen(context.Context, uint32) error                   { return nil }
func (*blockingReceiver) Idle(ctx context.Context) error                           { <-ctx.Done(); return ctx.Err() }

func (r *cancelOnFetchReceiver) Fetch(context.Context, int) ([]mailbox.RawMessage, error) {
	r.cancel()
	return nil, r.err
}
func (r *cancelOnFetchReceiver) MarkSeen(context.Context, uint32) error { return nil }
func (r *cancelOnFetchReceiver) Idle(context.Context) error             { return nil }

func TestRuntimeServesAuthenticatedWebConsoleWhenEnabled(t *testing.T) {
	d := t.TempDir()
	hash, err := webconsole.HashPassword("console-password", bytes.NewReader(bytes.Repeat([]byte{3}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(d, "mailrelay.yaml")
	body := fmt.Sprintf(`mail:
  imap: {address: "imap.example.com:993", username: u, password: p}
  smtp: {address: "smtp.example.com:465", username: u, password: p, from: relay@example.com}
security: {token: secret, allow: [me@example.com]}
storage: {path: relay.db}
web:
  enabled: true
  address: "127.0.0.1:0"
  session_secret: "12345678901234567890123456789012"
  admin_password_hash: %q
commands: []
`, hash)
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	runtime, err := Build(path)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	runtime.receiver = &blockingReceiver{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runtime.Run(ctx) }()
	t.Cleanup(func() { cancel(); <-done })

	deadline := time.Now().Add(2 * time.Second)
	for runtime.WebAddress() == "" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if runtime.WebAddress() == "" {
		t.Fatal("web listener did not start")
	}
	response, err := http.Post("http://"+runtime.WebAddress()+"/api/v1/login", "application/json", strings.NewReader(`{"password":"console-password"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	content, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK || !bytes.Contains(content, []byte(`"csrf"`)) {
		t.Fatalf("status=%d body=%s", response.StatusCode, content)
	}
}

func TestHotReloadIsAtomicAndKeepsLastValidConfig(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "mailrelay.yaml")
	write := func(desc string, valid bool) {
		body := `mail:
  imap: {address: "imap.example.com:993", username: u, password: p}
  smtp: {address: "smtp.example.com:465", username: u, password: p, from: relay@example.com}
security: {token: secret, allow: [me@example.com], http_hosts: [api.example.com]}
storage: {path: relay.db}
runtime: {command_timeout: 1s, config_reload: true}
commands:
  - name: push
    description: ` + desc + `
    handler: http
    config: {url: "https://api.example.com/push"}
`
		if !valid {
			body = "security: [invalid"
		}
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("old", true)
	r, err := Build(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	check := func(want string) {
		res, err := r.app.Execute(context.Background(), command.Request{Name: "help", Params: map[string]any{"command": "push"}})
		if err != nil || !strings.Contains(res.Body, want) {
			t.Fatalf("want=%q body=%q err=%v", want, res.Body, err)
		}
	}
	check("old")
	write("new", true)
	future := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(path, future, future)
	if err = r.reloadIfChanged(context.Background()); err != nil {
		t.Fatal(err)
	}
	check("new")
	write("broken", false)
	future = future.Add(2 * time.Second)
	_ = os.Chtimes(path, future, future)
	if err = r.reloadIfChanged(context.Background()); err == nil {
		t.Fatal("expected invalid reload")
	}
	events, err := r.store.RecentEvents(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 || events[0].Phase != "reload" || events[0].ErrorKind != "config" {
		t.Fatalf("events=%#v", events)
	}
	if events[0].Summary != "configuration reload rejected" {
		t.Fatalf("unexpected event summary: %#v", events[0])
	}
	if strings.Contains(events[0].Summary, "invalid") {
		t.Fatalf("raw reload error leaked into summary: %#v", events[0])
	}
	check("new")
}

func TestRunRecordsRejectedReloadOnce(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "mailrelay.yaml")
	body := `mail:
  imap: {address: "imap.example.com:993", username: u, password: p}
  smtp: {address: "smtp.example.com:465", username: u, password: p, from: relay@example.com}
security: {token: secret, allow: [me@example.com], http_hosts: [api.example.com]}
storage: {path: relay.db}
runtime: {command_timeout: 1s, config_reload: true}
commands:
  - name: push
    description: old
    handler: http
    config: {url: "https://api.example.com/push"}
`
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := Build(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := os.WriteFile(path, []byte("security: [invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(path, future, future)
	ctx, cancel := context.WithCancel(context.Background())
	r.receiver = &runCancelReceiver{cancel: cancel}
	if err := r.Run(ctx); err != nil {
		t.Fatal(err)
	}
	events, err := r.store.RecentEvents(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	var reloadEvent *store.RuntimeEvent
	for i := range events {
		if events[i].Phase == "reload" {
			reloadEvent = &events[i]
			break
		}
	}
	if reloadEvent == nil {
		t.Fatalf("expected reload event, got %#v", events)
	}
	if reloadEvent.Summary != "configuration reload rejected" {
		t.Fatalf("unexpected runtime event: %#v", reloadEvent)
	}
}

func TestRunSanitizesReceiverFailureEvent(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "mailrelay.yaml")
	body := `mail:
  imap: {address: "imap.example.com:993", username: u, password: p}
  smtp: {address: "smtp.example.com:465", username: u, password: p, from: relay@example.com}
security: {token: secret, allow: [me@example.com], http_hosts: [api.example.com]}
storage: {path: relay.db}
runtime: {command_timeout: 1s, config_reload: true}
commands: []
`
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := Build(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	rawErr := "imap login failed for vip@example.com token=topsecret"
	ctx, cancel := context.WithCancel(context.Background())
	r.receiver = &cancelOnFetchReceiver{err: errors.New(rawErr), cancel: cancel}
	if err := r.Run(ctx); err != nil {
		t.Fatal(err)
	}
	events, err := r.store.RecentEvents(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 {
		t.Fatal("expected receiver runtime event")
	}
	if events[0].Phase != "receiver" || events[0].ErrorKind != "dependency" || events[0].Summary != "mail receiver failed" {
		t.Fatalf("unexpected runtime event: %#v", events[0])
	}
	if strings.Contains(events[0].Summary, "vip@example.com") || strings.Contains(events[0].Summary, "topsecret") {
		t.Fatalf("raw receiver error leaked into summary: %#v", events[0])
	}
	state, err := r.store.State(context.Background(), "last_runtime_error")
	if err != nil {
		t.Fatal(err)
	}
	if state != "mail receiver failed" {
		t.Fatalf("unexpected last_runtime_error: %q", state)
	}
	if strings.Contains(state, "vip@example.com") || strings.Contains(state, "topsecret") {
		t.Fatalf("raw receiver error leaked into runtime state: %q", state)
	}
}

func TestRuntimeBuildWiresRetryPolicy(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "mailrelay.yaml")
	body := `mail:
  imap: {address: "imap.example.com:993", username: u, password: p}
  smtp: {address: "smtp.example.com:465", username: u, password: p, from: relay@example.com}
security: {token: secret, allow: [me@example.com], http_hosts: [api.example.com]}
storage: {path: relay.db}
runtime: {reply_max_attempts: 2, queue_max_attempts: 6, initial_backoff: 10s, max_backoff: 15s}
commands:
  - name: push
    handler: queue
    config: {command: deploy}
  - name: deploy
    handler: http
    config: {url: "https://api.example.com/deploy"}
`
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := Build(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if r.app.replyMaxAttempts != 2 || r.app.initialBackoff != 10*time.Second || r.app.maxBackoff != 15*time.Second {
		t.Fatalf("app retry policy not wired: %#v", r.app)
	}
	q, ok := r.registry.Get("queue")
	if !ok {
		t.Fatal("queue handler not registered")
	}
	queue, ok := q.(*handler.Queue)
	if !ok {
		t.Fatalf("unexpected queue handler type %T", q)
	}
	if queue.DefaultMaxAttempts() != 6 || queue.InitialBackoff() != 10*time.Second || queue.MaxBackoff() != 15*time.Second {
		t.Fatalf("queue retry policy not wired: max=%d initial=%s maxBackoff=%s", queue.DefaultMaxAttempts(), queue.InitialBackoff(), queue.MaxBackoff())
	}
}

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

func TestQueueRestartIdempotencyLifecycle(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "mailrelay.yaml")
	body := `mail:
  imap: {address: "imap.example.com:993", username: u, password: p}
  smtp: {address: "smtp.example.com:465", username: u, password: p, from: relay@example.com}
security: {token: secret, allow: [me@example.com], http_hosts: [api.example.com]}
storage: {path: relay.db}
runtime: {queue_max_attempts: 3}
commands:
  - name: later
    handler: queue
    parameters:
      env: {type: string, required: true}
    config: {command: deploy}
  - name: deploy
    handler: http
    parameters:
      env: {type: string, required: true}
    config: {url: "https://api.example.com/deploy"}
`
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := Build(path)
	if err != nil {
		t.Fatal(err)
	}
	req := command.Request{MessageID: "mail-queue", Name: "later", Params: map[string]any{"env": "prod"}}
	first, err := r.app.Execute(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := r.app.Execute(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if first.Data["job_id"] != second.Data["job_id"] {
		t.Fatalf("job ids differ: first=%v second=%v", first.Data, second.Data)
	}
	depth, err := r.store.QueueDepth(context.Background())
	if err != nil || depth != 1 {
		t.Fatalf("depth=%d err=%v", depth, err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := store.Open(filepath.Join(d, "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	job, err := reopened.ClaimJob(context.Background(), time.Now(), time.Minute)
	if err != nil || job == nil {
		t.Fatalf("job=%#v err=%v", job, err)
	}
	if job.Command != "deploy" || job.Params["env"] != "prod" || job.Attempts != 1 {
		t.Fatalf("recovered job=%#v", job)
	}
}

func TestWorkflowPartialFailureLifecycle(t *testing.T) {
	exec := &workflowLifecycleExec{}
	workflow := handler.NewWorkflow(10, 4)
	_, err := workflow.Execute(context.Background(), command.Context{
		Command: command.Command{Name: "release", Config: map[string]any{"steps": []any{
			map[string]any{"command": "build"},
			map[string]any{"command": "deploy"},
			map[string]any{"command": "notify"},
		}}},
		Request: command.Request{MessageID: "mail-workflow"},
		Execute: exec,
	})
	var commandErr *command.Error
	if !errors.As(err, &commandErr) || commandErr.Kind != "workflow_step" || commandErr.Message != "step 2 (deploy) failed" {
		t.Fatalf("error=%v", err)
	}
	if strings.Join(exec.names, ",") != "build,deploy" {
		t.Fatalf("executed=%v", exec.names)
	}
}

func TestMailConnectionError(t *testing.T) {
	cases := []struct {
		name      string
		err       error
		wantCause string
	}{
		{"dns", &net.DNSError{Name: "me@example.com", Err: "no such host"}, "dns"},
		{"timeout", &net.OpError{Op: "dial", Err: timeoutErr{}}, "timeout"},
		{"connect", &net.OpError{Op: "dial", Err: errors.New("connection refused")}, "connect"},
		{"tls", tls.RecordHeaderError{Msg: "first record does not look like a TLS handshake"}, "tls"},
		{"certificate", x509.UnknownAuthorityError{}, "certificate"},
		{"auth", errors.New("LOGIN failed: authentication error"), "auth"},
		{"unknown", errors.New("something unexpected"), "unknown"},
		{"nil", nil, "none"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cause, hint := mailConnectionError(tc.err)
			if cause != tc.wantCause {
				t.Fatalf("cause=%q want %q", cause, tc.wantCause)
			}
			if tc.err != nil && hint == "" {
				t.Fatal("expected a hint for a non-nil error")
			}
		})
	}
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func TestRunReceiverFailureLogIsActionableAndScrubbed(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	defer slog.SetDefault(prev)

	d := t.TempDir()
	path := filepath.Join(d, "mailrelay.yaml")
	body := `mail:
  imap: {address: "imap.example.com:993", username: u, password: p}
  smtp: {address: "smtp.example.com:465", username: u, password: p, from: relay@example.com}
security: {token: secret, allow: [me@example.com], http_hosts: [api.example.com]}
storage: {path: relay.db}
runtime: {command_timeout: 1s, config_reload: true}
commands: []
`
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := Build(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ctx, cancel := context.WithCancel(context.Background())
	r.receiver = &cancelOnFetchReceiver{err: errors.New("imap login failed for vip@example.com token=topsecret"), cancel: cancel}
	if err := r.Run(ctx); err != nil {
		t.Fatal(err)
	}
	logged := buf.String()
	if strings.Contains(logged, "topsecret") || strings.Contains(logged, "vip@example.com") {
		t.Fatalf("raw receiver error leaked into operator log: %q", logged)
	}
	for _, want := range []string{"mailrelay started", "mail poll failed", "cause=auth", "hint="} {
		if !strings.Contains(logged, want) {
			t.Fatalf("log missing %q; got %q", want, logged)
		}
	}
}
