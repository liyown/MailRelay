package security

import (
	"github.com/liyown/MailRelay/internal/command"
	"testing"
)

func TestAuthenticate(t *testing.T) {
	r := command.Request{Sender: "ME@example.com"}
	if err := Authenticate(r, "secret", []string{"me@example.com"}, "secret"); err != nil {
		t.Fatal(err)
	}
	if err := Authenticate(r, "bad", []string{"me@example.com"}, "secret"); err == nil {
		t.Fatal("expected denial")
	}
	if err := Authenticate(command.Request{Sender: "other@example.com"}, "secret", []string{"me@example.com"}, "secret"); err == nil {
		t.Fatal("expected sender denial")
	}
}
