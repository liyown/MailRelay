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

func TestParseRawHTTPRequestBody(t *testing.T) {
	raw := "From: me@example.com\r\nSubject: forward\r\nMessage-ID: <raw-http>\r\nX-MailRelay-Token: secret\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nGET /push/hello HTTP/1.1\r\nHost: api.example.com\r\nAccept: application/json\r\n\r\n"
	m, err := Parse(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if m.Request.Name != "forward" || m.Token != "secret" {
		t.Fatalf("unexpected message: %#v", m)
	}
	if !strings.Contains(m.Request.RawBody, "GET /push/hello HTTP/1.1") {
		t.Fatalf("raw body not preserved: %q", m.Request.RawBody)
	}
	if len(m.Request.Params) != 0 {
		t.Fatalf("raw HTTP request should not produce params: %#v", m.Request.Params)
	}
}
