package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/becomeopc/opc-mailrelay/internal/command"
	webconsole "github.com/becomeopc/opc-mailrelay/internal/web"
)

const editConfig = `mail:
  imap: {address: "imap.example.com:993", username: u, password: "mailpass-secret"}
  smtp: {address: "smtp.example.com:465", username: u, password: "mailpass-secret", from: relay@example.com}
security: {token: "tok-secret", allow: [me@example.com], http_hosts: [api.example.com]}
storage: {path: relay.db}
runtime: {command_timeout: 1s, config_reload: true}
commands:
  - name: push
    handler: http
    parameters: {message: {type: string, required: true}}
    config: {method: POST, url: "https://api.example.com/push", body: "{{message}}"}
`

func buildEditRuntime(t *testing.T) (*Runtime, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mailrelay.yaml")
	if err := os.WriteFile(path, []byte(editConfig), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := Build(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r, path
}

func commandNames(cmds []command.Command) []string {
	names := make([]string, 0, len(cmds))
	for _, c := range cmds {
		names = append(names, c.Name)
	}
	return names
}

func TestApplyDraftPersistsHotAppliesAndRefreshesRepo(t *testing.T) {
	r, path := buildEditRuntime(t)
	repo := r.newRepository(nil)

	draft, err := r.LoadDraft()
	if err != nil {
		t.Fatal(err)
	}
	draft.Commands = append(draft.Commands, command.Command{
		Name: "deploy", Description: "Deploy", Handler: "http",
		Parameters: map[string]command.Parameter{"service": {Type: "string", Required: true}},
		Config:     map[string]any{"method": "POST", "url": "https://api.example.com/deploy", "body": "{{service}}"},
	})
	draft.HTTPHosts = append(draft.HTTPHosts, "hooks.example.com")

	if err := r.ApplyDraft(context.Background(), draft); err != nil {
		t.Fatalf("apply: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"mailpass-secret", "tok-secret"} {
		if !strings.Contains(string(raw), secret) {
			t.Fatalf("secret %q dropped from persisted file:\n%s", secret, raw)
		}
	}
	if !strings.Contains(string(raw), "deploy") {
		t.Fatalf("new command not written:\n%s", raw)
	}

	r.mu.RLock()
	names := commandNames(r.cfg.Commands)
	hosts := append([]string(nil), r.cfg.Security.HTTPHosts...)
	r.mu.RUnlock()
	if !slices.Contains(names, "deploy") {
		t.Fatalf("live config missing deploy: %v", names)
	}
	if !slices.Contains(hosts, "hooks.example.com") {
		t.Fatalf("live http_hosts not updated: %v", hosts)
	}
	if got := repo.Commands(); len(got) != 2 {
		t.Fatalf("console repo command count=%d want 2", len(got))
	}
}

func TestApplyDraftRejectsInvalidDraftWithoutTouchingFile(t *testing.T) {
	r, path := buildEditRuntime(t)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	draft, err := r.LoadDraft()
	if err != nil {
		t.Fatal(err)
	}
	// An http command whose URL host is not allowlisted must fail Validate.
	draft.Commands = append(draft.Commands, command.Command{
		Name: "leak", Handler: "http",
		Config: map[string]any{"method": "POST", "url": "https://evil.example.net/x"},
	})
	err = r.ApplyDraft(context.Background(), draft)
	if err == nil {
		t.Fatal("expected the invalid draft to be rejected")
	}
	var de *webconsole.DraftError
	if !errors.As(err, &de) {
		t.Fatalf("want *webconsole.DraftError, got %T: %v", err, err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("config file changed despite the draft being rejected")
	}
}
