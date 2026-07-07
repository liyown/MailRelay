package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/becomeopc/opc-mailrelay/internal/command"
	"github.com/becomeopc/opc-mailrelay/internal/handler"
	"github.com/becomeopc/opc-mailrelay/internal/mailbox"
	"github.com/becomeopc/opc-mailrelay/internal/router"
	"github.com/becomeopc/opc-mailrelay/internal/store"
	"io"
	"strings"
	"testing"
	"time"
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
	err  error
}

type batchReceiver struct {
	msgs []mailbox.RawMessage
	seen []uint32
}

func (r *batchReceiver) Fetch(context.Context, int) ([]mailbox.RawMessage, error) { return r.msgs, nil }
func (r *batchReceiver) MarkSeen(_ context.Context, uid uint32) error {
	r.seen = append(r.seen, uid)
	return nil
}
func (r *batchReceiver) Idle(context.Context) error { return nil }

func (s *testSender) Send(_ context.Context, _ string, b []byte) error {
	s.n++
	s.body = b
	if s.err != nil {
		return s.err
	}
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

func TestOnceRecordsBadMessageAndContinuesBatch(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/batch.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h := &testHandler{}
	reg := handler.NewRegistry()
	_ = reg.Register(h)
	route, _ := router.New([]command.Command{{Name: "push", Handler: "test"}}, reg)
	sender := &testSender{}
	a := New(st, route, sender, "relay@example.com", []string{"me@example.com"}, "secret")
	recv := &batchReceiver{msgs: []mailbox.RawMessage{
		{UID: 1, Body: []byte("not a valid message")},
		{UID: 2, Body: []byte("From: me@example.com\r\nSubject: push\r\nMessage-ID: <good-batch>\r\nX-MailRelay-Token: secret\r\n\r\n")},
	}}
	if err := a.Once(context.Background(), recv, 100); err != nil {
		t.Fatal(err)
	}
	if h.calls != 1 || sender.n != 1 {
		t.Fatalf("handler calls=%d sends=%d", h.calls, sender.n)
	}
	if len(recv.seen) != 2 || recv.seen[0] != 1 || recv.seen[1] != 2 {
		t.Fatalf("seen=%v", recv.seen)
	}
	got, err := st.MessageState(context.Background(), "uid:1")
	if err != nil {
		t.Fatal(err)
	}
	if got.State != store.MessageParseFailed || got.ErrorKind != "parse" {
		t.Fatalf("unexpected bad message state: %#v", got)
	}
}

func TestOnceRecordsMalformedMessagesAsParseFailed(t *testing.T) {
	tests := []struct {
		name string
		body string
		uid  uint32
	}{
		{
			name: "empty subject",
			uid:  11,
			body: "From: me@example.com\r\nSubject:   \r\nMessage-ID: <bad-empty-subject>\r\nX-MailRelay-Token: secret\r\n\r\n",
		},
		{
			name: "invalid JSON body",
			uid:  12,
			body: "From: me@example.com\r\nSubject: push\r\nMessage-ID: <bad-json>\r\nX-MailRelay-Token: secret\r\nContent-Type: application/json\r\n\r\n{\"oops\":",
		},
		{
			name: "invalid body line",
			uid:  13,
			body: "From: me@example.com\r\nSubject: push\r\nMessage-ID: <bad-body-line>\r\nX-MailRelay-Token: secret\r\n\r\nnot-an-assignment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, err := store.Open(t.TempDir() + "/malformed.db")
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			h := &testHandler{}
			reg := handler.NewRegistry()
			_ = reg.Register(h)
			route, _ := router.New([]command.Command{{Name: "push", Handler: "test"}}, reg)
			sender := &testSender{}
			a := New(st, route, sender, "relay@example.com", []string{"me@example.com"}, "secret")
			recv := &batchReceiver{msgs: []mailbox.RawMessage{
				{UID: tt.uid, Body: []byte(tt.body)},
				{UID: 99, Body: []byte("From: me@example.com\r\nSubject: push\r\nMessage-ID: <good-after-bad>\r\nX-MailRelay-Token: secret\r\n\r\n")},
			}}

			if err := a.Once(context.Background(), recv, 100); err != nil {
				t.Fatal(err)
			}
			got, err := st.MessageState(context.Background(), fmt.Sprintf("uid:%d", tt.uid))
			if err != nil {
				t.Fatal(err)
			}
			if got.State != store.MessageParseFailed || got.ErrorKind != "parse" {
				t.Fatalf("unexpected malformed message state: %#v", got)
			}
			if h.calls != 1 || sender.n != 1 {
				t.Fatalf("handler calls=%d sends=%d", h.calls, sender.n)
			}
			if len(recv.seen) != 2 || recv.seen[0] != tt.uid || recv.seen[1] != 99 {
				t.Fatalf("seen=%v", recv.seen)
			}
		})
	}
}

func TestOnceAuthFailureUsesMessageIDWithoutDeadUIDDuplicate(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/auth.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h := &testHandler{}
	reg := handler.NewRegistry()
	_ = reg.Register(h)
	route, _ := router.New([]command.Command{{Name: "push", Handler: "test"}}, reg)
	sender := &testSender{}
	a := New(st, route, sender, "relay@example.com", []string{"me@example.com"}, "secret")
	recv := &batchReceiver{msgs: []mailbox.RawMessage{
		{UID: 7, Body: []byte("From: me@example.com\r\nSubject: push\r\nMessage-ID: <auth-bad>\r\nX-MailRelay-Token: wrong\r\n\r\n")},
		{UID: 8, Body: []byte("From: me@example.com\r\nSubject: push\r\nMessage-ID: <auth-good>\r\nX-MailRelay-Token: secret\r\n\r\n")},
	}}
	if err := a.Once(context.Background(), recv, 100); err != nil {
		t.Fatal(err)
	}
	got, err := st.MessageState(context.Background(), "auth-bad")
	if err != nil {
		t.Fatal(err)
	}
	if got.State != store.MessageAuthFailed || got.ErrorKind != "authentication" {
		t.Fatalf("unexpected auth failure state: %#v", got)
	}
	_, err = st.MessageState(context.Background(), "uid:7")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected no uid dead record, err=%v", err)
	}
	if h.calls != 1 || sender.n != 1 {
		t.Fatalf("handler calls=%d sends=%d", h.calls, sender.n)
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

func TestRunOneReplyRedactsStoredSMTPFailure(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/reply-redact.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	rawErr := "550 rcpt <vip@example.com> rejected; sasl token=topsecret"
	sender := &testSender{err: errors.New(rawErr)}
	a := New(st, nil, sender, "relay@example.com", nil, "")
	a.replyBackoff = 0
	if _, err := st.RecordExecutionAndReply(
		context.Background(),
		store.Execution{MessageID: "reply-redact-1", Command: "push", Handler: "test", Status: "success", StartedAt: time.Now()},
		nil,
		"vip@example.com",
		[]byte("reply"),
		1,
	); err != nil {
		t.Fatal(err)
	}

	worked, err := a.RunOneReply(context.Background())
	if err != nil || !worked {
		t.Fatalf("worked=%v err=%v", worked, err)
	}

	state, err := st.MessageState(context.Background(), "reply-redact-1")
	if err != nil {
		t.Fatal(err)
	}
	if state.State != store.MessageDead || state.ErrorKind != "reply_delivery" || state.ErrorSummary != "delivery failed" {
		t.Fatalf("unexpected message state: %#v", state)
	}
	if strings.Contains(state.ErrorSummary, "vip@example.com") || strings.Contains(state.ErrorSummary, "topsecret") {
		t.Fatalf("raw SMTP data leaked into processed_messages: %#v", state)
	}

	failure, err := st.LatestFailure(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if failure != "reply: delivery failed" {
		t.Fatalf("unexpected latest failure: %q", failure)
	}
	if strings.Contains(failure, "vip@example.com") || strings.Contains(failure, "topsecret") {
		t.Fatalf("raw SMTP data leaked into outbox failure: %q", failure)
	}

	events, err := st.RecentEvents(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 {
		t.Fatal("expected runtime event")
	}
	if events[0].Phase != "reply" || events[0].ErrorKind != "dependency" || events[0].Summary != "reply delivery failed" {
		t.Fatalf("unexpected runtime event: %#v", events[0])
	}
	if strings.Contains(events[0].Summary, "vip@example.com") || strings.Contains(events[0].Summary, "topsecret") {
		t.Fatalf("raw SMTP data leaked into runtime event: %#v", events[0])
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
