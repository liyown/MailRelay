package config

import (
	"github.com/becomeopc/opc-mailrelay/internal/command"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateRejectsInvalidCommandGraphs(t *testing.T) {
	base := func(commands ...command.Command) Config {
		return Config{Security: Security{Token: "secret", Allow: []string{"me@example.com"}}, Commands: commands}
	}
	cases := []struct {
		name string
		cfg  Config
		want string
	}{
		{
			name: "workflow missing target",
			cfg:  base(command.Command{Name: "release", Handler: "workflow", Config: map[string]any{"steps": []any{map[string]any{"command": "missing"}}}}),
			want: "workflow target missing",
		},
		{
			name: "queue missing target",
			cfg:  base(command.Command{Name: "later", Handler: "queue", Config: map[string]any{"command": "missing"}}),
			want: "queue target missing",
		},
		{
			name: "indirect workflow cycle",
			cfg: base(
				command.Command{Name: "a", Handler: "workflow", Config: map[string]any{"steps": []any{map[string]any{"command": "b"}}}},
				command.Command{Name: "b", Handler: "workflow", Config: map[string]any{"steps": []any{map[string]any{"command": "a"}}}},
			),
			want: "command cycle",
		},
		{
			name: "mixed queue workflow cycle",
			cfg: base(
				command.Command{Name: "a", Handler: "queue", Config: map[string]any{"command": "b"}},
				command.Command{Name: "b", Handler: "workflow", Config: map[string]any{"steps": []any{map[string]any{"command": "a"}}}},
			),
			want: "command cycle",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.cfg.Validate(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() error=%v, want %q", err, tc.want)
			}
		})
	}
}

func TestLoadExpandsAndDescribesCommands(t *testing.T) {
	t.Setenv("TOKEN", "secret")
	p := filepath.Join(t.TempDir(), "mailrelay.yaml")
	y := `mail:
  imap: {address: "imap.example.com:993", username: relay, password: pass}
  smtp: {address: "smtp.example.com:465", username: relay, password: pass, from: relay@example.com}
security: {token: "${TOKEN}", allow: [me@example.com], http_hosts: [api.example.com]}
storage: {path: relay.db}
commands:
  - name: push
    description: Push message
    handler: http
    parameters:
      message: {description: Text, type: string, required: true, example: hello}
    config: {url: "https://api.example.com/push", method: POST}
`
	if err := os.WriteFile(p, []byte(y), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Security.Token != "secret" || c.Commands[0].Description != "Push message" || c.Commands[0].Parameters["message"].Example != "hello" {
		t.Fatalf("unexpected config: %#v", c)
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	cases := []string{
		"security: {token: '${MISSING}'}\n",
		"security: {token: x, allow: [a@b]}\ncommands: [{name: help, handler: http}]\n",
		"security: {token: x, allow: [a@b]}\ncommands: [{name: x, handler: shell, config: {executable: relative}}]\n",
		"security: {token: x, allow: [a@b], surprise: true}\n",
	}
	for _, body := range cases {
		t.Run(strings.Split(body, "\n")[0], func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "c.yaml")
			if err := os.WriteFile(p, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(p); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

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

func TestRuntimeConfigReloadDefaultsTrueWhenRuntimeOmitted(t *testing.T) {
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
	if !c.Runtime.ConfigReload {
		t.Fatalf("expected config_reload default true when runtime omitted, got %#v", c.Runtime)
	}
}
