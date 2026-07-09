package config

import (
	"strings"
	"testing"

	"github.com/becomeopc/opc-mailrelay/internal/command"
)

const draftOriginal = `mail:
  imap: {address: "imap.qq.com:993", username: "u@qq.com", password: "supersecretpass", mailbox: INBOX}
  smtp: {address: "smtp.qq.com:465", username: "u@qq.com", password: "supersecretpass", from: "u@qq.com"}
security:
  token: "topsecret-token"
  allow: ["u@qq.com"]
  http_hosts: ["api.example.com"]
storage: {path: "data/mailrelay.db"}
web:
  enabled: true
  session_secret: "session-secret-value-abc"
  admin_password_hash: "$argon2id$v=19$..."
runtime:
  command_timeout: 30s
  catalog_notify: ["u@qq.com"]
# a trailing comment outside the edited sections
commands:
  - name: push
    handler: http
    config: {method: POST, url: "https://api.example.com/push", token: "${PUSH_TOKEN}"}
`

func TestParseDraftKeepsEnvTokensUnresolved(t *testing.T) {
	d, err := ParseDraft([]byte(draftOriginal))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Commands) != 1 || d.Commands[0].Name != "push" {
		t.Fatalf("commands=%#v", d.Commands)
	}
	if got := d.Commands[0].Config["token"]; got != "${PUSH_TOKEN}" {
		t.Fatalf("env token resolved or lost: %v", got)
	}
	if len(d.HTTPHosts) != 1 || d.HTTPHosts[0] != "api.example.com" {
		t.Fatalf("hosts=%#v", d.HTTPHosts)
	}
	if len(d.CatalogNotify) != 1 || d.CatalogNotify[0] != "u@qq.com" {
		t.Fatalf("notify=%#v", d.CatalogNotify)
	}
}

func TestRenderDraftReplacesOnlyEditableSectionsAndPreservesSecrets(t *testing.T) {
	draft := Draft{
		Commands: []command.Command{
			{Name: "deploy", Description: "Deploy", Handler: "http", Config: map[string]any{"method": "POST", "url": "https://api.example.com/deploy", "token": "${DEPLOY_TOKEN}"}},
		},
		HTTPHosts:     []string{"api.example.com", "hooks.example.com"},
		CatalogNotify: []string{"ops@qq.com"},
	}
	out, err := RenderDraft([]byte(draftOriginal), draft)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, secret := range []string{"supersecretpass", "topsecret-token", "session-secret-value-abc", "$argon2id$v=19$..."} {
		if !strings.Contains(s, secret) {
			t.Fatalf("secret %q dropped:\n%s", secret, s)
		}
	}
	if !strings.Contains(s, "${DEPLOY_TOKEN}") {
		t.Fatalf("env token in new command not preserved:\n%s", s)
	}
	if strings.Contains(s, "push") {
		t.Fatalf("old command was not replaced:\n%s", s)
	}
	if !strings.Contains(s, "imap.qq.com:993") {
		t.Fatalf("untouched mail section altered:\n%s", s)
	}
	d, err := ParseDraft(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Commands) != 1 || d.Commands[0].Name != "deploy" {
		t.Fatalf("commands=%#v", d.Commands)
	}
	if len(d.HTTPHosts) != 2 {
		t.Fatalf("hosts=%#v", d.HTTPHosts)
	}
	if len(d.CatalogNotify) != 1 || d.CatalogNotify[0] != "ops@qq.com" {
		t.Fatalf("notify=%#v", d.CatalogNotify)
	}
}

func TestRenderDraftCreatesMissingRuntimeSection(t *testing.T) {
	original := `security: {token: t, allow: [a@b.c]}
storage: {path: x.db}
`
	out, err := RenderDraft([]byte(original), Draft{CatalogNotify: []string{"a@b.c"}, HTTPHosts: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	d, err := ParseDraft(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.CatalogNotify) != 1 || d.CatalogNotify[0] != "a@b.c" {
		t.Fatalf("catalog_notify not written into a new runtime section: %s", out)
	}
}
