package app

import (
	"context"
	"github.com/becomeopc/opc-mailrelay/internal/command"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
	check("new")
}
