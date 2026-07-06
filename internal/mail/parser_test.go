package mail

import (
	"strings"
	"testing"
)

func TestParsePlainCommand(t *testing.T) {
	raw := "From: User <ME@example.com>\r\nSubject: push\r\nMessage-ID: <abc@example.com>\r\nX-MailRelay-Token: header\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nmessage=hello\n_token=body\n"
	m, err := Parse(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if m.Request.Name != "push" || m.Request.Sender != "me@example.com" || m.Request.MessageID != "abc@example.com" || m.Token != "header" || m.Request.Params["message"] != "hello" {
		t.Fatalf("%#v", m)
	}
}
func TestParseHelpAndJSON(t *testing.T) {
	raw := "From: me@example.com\r\nSubject: help deploy\r\nContent-Type: application/json\r\n\r\n{\"_token\":\"x\"}"
	m, err := Parse(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if m.Request.Name != "help" || m.Request.Params["command"] != "deploy" || m.Token != "x" || m.Request.MessageID == "" {
		t.Fatalf("%#v", m)
	}
}
