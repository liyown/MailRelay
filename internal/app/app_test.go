package app

import (
	"context"
	"github.com/becomeopc/opc-mailrelay/internal/command"
	"github.com/becomeopc/opc-mailrelay/internal/handler"
	"github.com/becomeopc/opc-mailrelay/internal/router"
	"github.com/becomeopc/opc-mailrelay/internal/store"
	"io"
	"strings"
	"testing"
)

type testHandler struct{ calls int }

func (h *testHandler) Name() string { return "test" }
func (h *testHandler) Execute(context.Context, command.Context) (command.Result, error) {
	h.calls++
	return command.Result{Status: "success", Summary: "done", Body: "ok"}, nil
}

type testSender struct {
	n    int
	body []byte
}

func (s *testSender) Send(_ context.Context, _ string, b []byte) error       { s.n++; s.body = b; return nil }
func (s *testSender) Notify(context.Context, []string, string, string) error { return nil }
func TestProcessAuthenticatesAndDeduplicates(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/x.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h := &testHandler{}
	reg := handler.NewRegistry()
	_ = reg.Register(h)
	r, _ := router.New([]command.Command{{Name: "push", Description: "Push", Handler: "test", Parameters: map[string]command.Parameter{"message": {Type: "string", Required: true}}}}, reg)
	sender := &testSender{}
	a := New(st, r, sender, "relay@example.com", []string{"me@example.com"}, "secret")
	raw := "From: me@example.com\r\nSubject: push\r\nMessage-ID: <m1>\r\nX-MailRelay-Token: secret\r\n\r\nmessage=hello\n"
	for i := 0; i < 2; i++ {
		if err = a.Process(context.Background(), io.NopCloser(strings.NewReader(raw))); err != nil {
			t.Fatal(err)
		}
	}
	if h.calls != 1 || sender.n != 1 {
		t.Fatalf("calls=%d sends=%d", h.calls, sender.n)
	}
}
