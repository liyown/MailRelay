package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
