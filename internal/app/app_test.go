package app

import (
	"context"
	"fmt"
	"github.com/liyown/MailRelay/internal/command"
	"github.com/liyown/MailRelay/internal/handler"
	"github.com/liyown/MailRelay/internal/router"
	"github.com/liyown/MailRelay/internal/store"
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
	fail int
}

func (s *testSender) Send(_ context.Context, _ string, b []byte) error {
	s.n++
	s.body = b
	if s.fail > 0 {
		s.fail--
		return fmt.Errorf("smtp unavailable")
	}
	return nil
}
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
	a.replyBackoff = 0
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

func TestReplyRetryDoesNotExecuteHandlerTwice(t *testing.T) {
	path := t.TempDir() + "/retry.db"
	st, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	h := &testHandler{}
	reg := handler.NewRegistry()
	_ = reg.Register(h)
	r, _ := router.New([]command.Command{{Name: "push", Handler: "test"}}, reg)
	sender := &testSender{fail: 1}
	a := New(st, r, sender, "relay@example.com", []string{"me@example.com"}, "secret")
	a.replyBackoff = 0
	raw := "From: me@example.com\r\nSubject: push\r\nMessage-ID: <retry-1>\r\nX-MailRelay-Token: secret\r\n\r\n"
	if err = a.Process(context.Background(), io.NopCloser(strings.NewReader(raw))); err != nil {
		t.Fatal(err)
	}
	if h.calls != 1 {
		t.Fatalf("calls=%d", h.calls)
	}
	_ = st.Close()
	st, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a.store = st
	worked, err := a.RunOneReply(context.Background())
	if err != nil || !worked {
		t.Fatalf("worked=%v err=%v", worked, err)
	}
	if h.calls != 1 || sender.n != 2 {
		t.Fatalf("handler=%d sends=%d", h.calls, sender.n)
	}
}

func TestAuthenticatedHelpMailReturnsGeneratedCatalog(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/help.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h := &testHandler{}
	reg := handler.NewRegistry()
	_ = reg.Register(h)
	r, err := router.New([]command.Command{{Name: "deploy", Description: "Deploy application", Handler: "test", Parameters: map[string]command.Parameter{"env": {Description: "Environment", Type: "string", Required: true, Example: "prod"}}}}, reg)
	if err != nil {
		t.Fatal(err)
	}
	sender := &testSender{}
	a := New(st, r, sender, "relay@example.com", []string{"me@example.com"}, "secret")
	raw := "From: me@example.com\r\nSubject: help deploy\r\nMessage-ID: <help-1>\r\nX-MailRelay-Token: secret\r\n\r\n"
	if err = a.Process(context.Background(), io.NopCloser(strings.NewReader(raw))); err != nil {
		t.Fatal(err)
	}
	got := string(sender.body)
	for _, want := range []string{"Deploy application", "Environment", "prod", "In-Reply-To: <help-1>"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in reply:\n%s", want, got)
		}
	}
	if h.calls != 0 {
		t.Fatalf("help must not invoke handler, calls=%d", h.calls)
	}
}
