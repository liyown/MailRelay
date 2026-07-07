package app

import (
	"context"
	"errors"
	"github.com/becomeopc/opc-mailrelay/internal/command"
	"github.com/becomeopc/opc-mailrelay/internal/mailbox"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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

func (r *failingReceiver) Fetch(context.Context, int) ([]mailbox.RawMessage, error) { return nil, r.err }
func (r *failingReceiver) MarkSeen(context.Context, uint32) error                    { return nil }
func (r *failingReceiver) Idle(context.Context) error                                { return nil }

type cancelOnFetchReceiver struct {
	err    error
	cancel context.CancelFunc
}

func (r *cancelOnFetchReceiver) Fetch(context.Context, int) ([]mailbox.RawMessage, error) {
	r.cancel()
	return nil, r.err
}
func (r *cancelOnFetchReceiver) MarkSeen(context.Context, uint32) error { return nil }
func (r *cancelOnFetchReceiver) Idle(context.Context) error             { return nil }

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
	if len(events) != 1 {
		t.Fatalf("expected 1 reload event, got %#v", events)
	}
	if events[0].Phase != "reload" || events[0].Summary != "configuration reload rejected" {
		t.Fatalf("unexpected runtime event: %#v", events[0])
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
}
