package app

import (
	"context"
	"database/sql"
	"encoding/json"
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

func TestPreviewMailValidatesWithoutExecuting(t *testing.T) {
	h := &testHandler{}
	reg := handler.NewRegistry()
	if err := reg.Register(h); err != nil {
		t.Fatal(err)
	}
	route, err := router.New([]command.Command{{
		Name: "push", Handler: "test",
		Parameters: map[string]command.Parameter{"message": {Type: "string", Required: true}},
	}}, reg)
	if err != nil {
		t.Fatal(err)
	}
	a := New(nil, route, nil, "relay@example.com", []string{"me@example.com"}, "secret")
	accepted := a.PreviewMail("From: me@example.com\nSubject: push\n\n_token=secret\nmessage=hello")
	if !accepted.Accepted || accepted.Handler != "test" || h.calls != 0 {
		t.Fatalf("preview=%#v calls=%d", accepted, h.calls)
	}
	badToken := a.PreviewMail("From: me@example.com\nSubject: push\n\n_token=wrong\nmessage=hello")
	if badToken.Accepted || badToken.ErrorKind != "token" {
		t.Fatalf("token preview=%#v", badToken)
	}
	badParams := a.PreviewMail("From: me@example.com\nSubject: push\n\n_token=secret")
	if badParams.Accepted || badParams.ErrorKind != "invalid_parameters" {
		t.Fatalf("params preview=%#v", badParams)
	}
}

type paramsCaptureHandler struct {
	calls int
	got   map[string]any
}

func (h *paramsCaptureHandler) Name() string { return "capture" }
func (h *paramsCaptureHandler) Execute(_ context.Context, x command.Context) (command.Result, error) {
	h.calls++
	h.got = x.Request.Params
	return command.Result{Status: "success", Summary: "captured", Body: "ok"}, nil
}

type closeStoreHandler struct {
	calls int
	store *store.Store
}

func (h *closeStoreHandler) Name() string { return "closer" }
func (h *closeStoreHandler) Execute(context.Context, command.Context) (command.Result, error) {
	h.calls++
	_ = h.store.Close()
	return command.Result{Status: "success", Summary: "done", Body: "ok"}, nil
}

type failingHandler struct{ err error }

func (h *failingHandler) Name() string { return "failing" }
func (h *failingHandler) Execute(context.Context, command.Context) (command.Result, error) {
	return command.Result{}, h.err
}

type testSender struct {
	n      int
	body   []byte
	bodies [][]byte
	fail   int
	err    error
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
	s.bodies = append(s.bodies, append([]byte(nil), b...))
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
	a.SetRetryPolicy(5, 0, time.Minute)
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
			if h.calls != 1 || sender.n != 2 {
				t.Fatalf("handler calls=%d sends=%d", h.calls, sender.n)
			}
			if !strings.Contains(string(sender.bodies[0]), "MailRelay command rejected") || !strings.Contains(string(sender.bodies[0]), "parse failed") {
				t.Fatalf("parse failure reply missing safe reason:\n%s", sender.bodies)
			}
			if len(recv.seen) != 2 || recv.seen[0] != tt.uid || recv.seen[1] != 99 {
				t.Fatalf("seen=%v", recv.seen)
			}
			exec, err := st.RecentExecution(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if exec.Status != "success" {
				t.Fatalf("latest execution should be good message, got %#v", exec)
			}
		})
	}
}

func TestOnceDoesNotReplyToSelfGeneratedParseFailure(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/self-parse.db")
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
		{UID: 26, Body: []byte("From: relay@example.com\r\nSubject: [MailRelay error] Command\r\nMessage-ID: <self-error>\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nMailRelay command rejected\r\n\r\nYour command was not accepted: parse failed.\r\n")},
	}}

	if err := a.Once(context.Background(), recv, 100); err != nil {
		t.Fatal(err)
	}
	if h.calls != 0 || sender.n != 0 {
		t.Fatalf("handler calls=%d sends=%d", h.calls, sender.n)
	}
	if len(recv.seen) != 1 || recv.seen[0] != 26 {
		t.Fatalf("seen=%v", recv.seen)
	}
	got, err := st.MessageState(context.Background(), "uid:26")
	if err != nil {
		t.Fatal(err)
	}
	if got.State != store.MessageParseFailed || got.ErrorKind != "parse" {
		t.Fatalf("unexpected self parse state: %#v", got)
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
	if got.State != store.MessageAuthFailed || got.ErrorKind != "authentication" || got.ErrorSummary != "invalid token" {
		t.Fatalf("unexpected auth failure state: %#v", got)
	}
	_, err = st.MessageState(context.Background(), "uid:7")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected no uid dead record, err=%v", err)
	}
	if h.calls != 1 || sender.n != 2 {
		t.Fatalf("handler calls=%d sends=%d", h.calls, sender.n)
	}
	if len(sender.bodies) < 1 || !strings.Contains(string(sender.bodies[0]), "MailRelay command rejected") || !strings.Contains(string(sender.bodies[0]), "invalid token") {
		t.Fatalf("auth failure reply missing safe reason:\n%s", sender.bodies)
	}
	events, err := st.RecentEvents(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	var sawAuth bool
	for _, event := range events {
		if event.Phase == "auth" && event.ErrorKind == "authentication" && event.Summary == "invalid token" {
			sawAuth = true
		}
		if strings.Contains(event.Summary, "wrong") {
			t.Fatalf("raw token leaked into event: %#v", event)
		}
	}
	if !sawAuth {
		t.Fatalf("missing auth event: %#v", events)
	}
	exec, err := st.RecentExecution(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if exec.MessageID != "auth-good" || exec.Status != "success" {
		t.Fatalf("latest execution should be successful command, got %#v", exec)
	}
}

func TestProcessAuthFailureRecordsExecutionAndReplies(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/auth-execution.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	reg := handler.NewRegistry()
	_ = reg.Register(&testHandler{})
	route, _ := router.New([]command.Command{{Name: "push", Handler: "test"}}, reg)
	sender := &testSender{}
	a := New(st, route, sender, "relay@example.com", []string{"me@example.com"}, "secret")
	raw := "From: me@example.com\r\nSubject: push\r\nMessage-ID: <auth-execution>\r\nX-MailRelay-Token: wrong\r\n\r\n"

	err = a.Process(context.Background(), io.NopCloser(strings.NewReader(raw)))
	if err == nil {
		t.Fatal("expected auth failure")
	}
	exec, err := st.RecentExecution(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if exec.MessageID != "auth-execution" || exec.Command != "push" || exec.Handler != "auth" || exec.Status != "error" || exec.Error != "authentication" || exec.Summary != "invalid token" {
		t.Fatalf("unexpected auth execution: %#v", exec)
	}
	if sender.n != 1 || !strings.Contains(string(sender.body), "invalid token") {
		t.Fatalf("missing auth failure reply sends=%d body=%s", sender.n, sender.body)
	}
}

func TestProcessHandlerFailureRecordsSanitizedDetail(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/handler-detail.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	reg := handler.NewRegistry()
	rawErr := "dial tcp vip@example.com failed with token=topsecret"
	_ = reg.Register(&failingHandler{err: &command.Error{Kind: "dependency", Message: "HTTP request failed", Err: errors.New(rawErr)}})
	route, _ := router.New([]command.Command{{Name: "push", Handler: "failing"}}, reg)
	sender := &testSender{}
	a := New(st, route, sender, "relay@example.com", []string{"me@example.com"}, "secret")
	raw := "From: me@example.com\r\nSubject: push\r\nMessage-ID: <handler-detail>\r\nX-MailRelay-Token: secret\r\n\r\n"

	if err := a.Process(context.Background(), io.NopCloser(strings.NewReader(raw))); err != nil {
		t.Fatal(err)
	}

	exec, err := st.RecentExecution(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if exec.Status != "error" || exec.Error != "dependency" {
		t.Fatalf("unexpected execution: %#v", exec)
	}
	if !strings.Contains(exec.Summary, "HTTP request failed") || !strings.Contains(exec.Summary, "[email]") || !strings.Contains(exec.Summary, "[REDACTED]") {
		t.Fatalf("execution summary missing sanitized detail: %#v", exec)
	}
	if strings.Contains(exec.Summary, "vip@example.com") || strings.Contains(exec.Summary, "topsecret") {
		t.Fatalf("raw detail leaked into execution summary: %#v", exec)
	}
	events, err := st.RecentEvents(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 || !strings.Contains(events[0].Summary, "HTTP request failed") {
		t.Fatalf("missing handler event detail: %#v", events)
	}
	if strings.Contains(events[0].Summary, "vip@example.com") || strings.Contains(events[0].Summary, "topsecret") {
		t.Fatalf("raw detail leaked into runtime event: %#v", events[0])
	}
}

func TestOnceDoesNotMarkSeenWhenDurabilityFailsAfterHandler(t *testing.T) {
	path := t.TempDir() + "/durability.db"
	st, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	h := &closeStoreHandler{store: st}
	reg := handler.NewRegistry()
	_ = reg.Register(h)
	route, _ := router.New([]command.Command{{Name: "push", Handler: "closer"}}, reg)
	a := New(st, route, &testSender{}, "relay@example.com", []string{"me@example.com"}, "secret")
	recv := &batchReceiver{msgs: []mailbox.RawMessage{
		{UID: 42, Body: []byte("From: me@example.com\r\nSubject: push\r\nMessage-ID: <durability-fail>\r\nX-MailRelay-Token: secret\r\n\r\n")},
	}}

	if err := a.Once(context.Background(), recv, 100); err == nil {
		t.Fatal("expected durability failure")
	}
	if h.calls != 1 {
		t.Fatalf("expected handler to run once before durability failure, calls=%d", h.calls)
	}
	if len(recv.seen) != 0 {
		t.Fatalf("durability failure must not mark message seen, seen=%v", recv.seen)
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
	a.SetRetryPolicy(5, time.Nanosecond, time.Nanosecond)
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

func TestProcessRedactsSensitiveParamsInExecutionButHandlerSeesRaw(t *testing.T) {
	path := t.TempDir() + "/redact-exec.db"
	st, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h := &paramsCaptureHandler{}
	reg := handler.NewRegistry()
	_ = reg.Register(h)
	cmd := command.Command{
		Name:    "deploy",
		Handler: "capture",
		Parameters: map[string]command.Parameter{
			"env":    {Type: "string", Required: true},
			"secret": {Type: "string", Required: true, Sensitive: true},
		},
	}
	route, _ := router.New([]command.Command{cmd}, reg)
	sender := &testSender{}
	a := New(st, route, sender, "relay@example.com", []string{"me@example.com"}, "secret")
	raw := "From: me@example.com\r\nSubject: deploy\r\nMessage-ID: <redact-exec>\r\nX-MailRelay-Token: secret\r\n\r\nenv=prod\nsecret=s3cr3t\n"

	if err := a.Process(context.Background(), io.NopCloser(strings.NewReader(raw))); err != nil {
		t.Fatal(err)
	}
	if h.got["secret"] != "s3cr3t" {
		t.Fatalf("handler did not receive raw secret params: %#v", h.got)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var rawParams string
	if err := db.QueryRow(`SELECT params_json FROM executions WHERE message_id='redact-exec'`).Scan(&rawParams); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(rawParams), &got); err != nil {
		t.Fatal(err)
	}
	if got["secret"] != "[REDACTED]" || got["env"] != "prod" {
		t.Fatalf("unexpected persisted params: %s", rawParams)
	}
	if strings.Contains(rawParams, "s3cr3t") {
		t.Fatalf("raw secret leaked into execution params: %s", rawParams)
	}
}

func TestProcessQueueWrapperRedactsTargetSensitiveParamsInExecution(t *testing.T) {
	path := t.TempDir() + "/queue-wrapper-redact.db"
	st, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	q := handler.NewQueue(st)
	reg := handler.NewRegistry()
	if err := reg.Register(q); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(&paramsCaptureHandler{}); err != nil {
		t.Fatal(err)
	}
	route, err := router.New([]command.Command{
		{
			Name:    "later",
			Handler: "queue",
			Config:  map[string]any{"command": "deploy"},
			Parameters: map[string]command.Parameter{
				"env":    {Type: "string", Required: true},
				"secret": {Type: "string", Required: true},
			},
		},
		{
			Name:    "deploy",
			Handler: "capture",
			Parameters: map[string]command.Parameter{
				"env":    {Type: "string", Required: true},
				"secret": {Type: "string", Required: true, Sensitive: true},
			},
		},
	}, reg)
	if err != nil {
		t.Fatal(err)
	}
	a := New(st, route, &testSender{}, "relay@example.com", []string{"me@example.com"}, "secret")
	raw := "From: me@example.com\r\nSubject: later\r\nMessage-ID: <queue-wrapper-redact>\r\nX-MailRelay-Token: secret\r\n\r\nenv=prod\nsecret=queued-secret\n"

	if err := a.Process(context.Background(), io.NopCloser(strings.NewReader(raw))); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var execRaw, queueRaw string
	if err := db.QueryRow(`SELECT params_json FROM executions WHERE message_id='queue-wrapper-redact'`).Scan(&execRaw); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT params_json FROM queue_jobs WHERE idempotency_key='queue-wrapper-redact:later'`).Scan(&queueRaw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(execRaw, "queued-secret") {
		t.Fatalf("raw target secret leaked into execution params: %s", execRaw)
	}
	if strings.Contains(queueRaw, "queued-secret") {
		t.Fatalf("raw target secret leaked into queue params: %s", queueRaw)
	}
	var execParams map[string]any
	if err := json.Unmarshal([]byte(execRaw), &execParams); err != nil {
		t.Fatal(err)
	}
	if execParams["secret"] != "[REDACTED]" || execParams["env"] != "prod" {
		t.Fatalf("unexpected execution params: %s", execRaw)
	}
}

func TestReplyFailureUsesConfiguredAttemptsAndBackoff(t *testing.T) {
	path := t.TempDir() + "/reply-policy.db"
	st, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h := &testHandler{}
	reg := handler.NewRegistry()
	_ = reg.Register(h)
	route, _ := router.New([]command.Command{{Name: "push", Handler: "test"}}, reg)
	sender := &testSender{err: errors.New("smtp token=secret unavailable")}
	a := New(st, route, sender, "relay@example.com", []string{"me@example.com"}, "secret")
	a.SetRetryPolicy(2, 10*time.Second, 15*time.Second)
	before := time.Now()
	raw := "From: me@example.com\r\nSubject: push\r\nMessage-ID: <reply-policy>\r\nX-MailRelay-Token: secret\r\n\r\n"

	if err := a.Process(context.Background(), io.NopCloser(strings.NewReader(raw))); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var maxAttempts int
	var availableRaw string
	if err := db.QueryRow(`SELECT max_attempts,available_at FROM outbox_replies WHERE message_id='reply-policy'`).Scan(&maxAttempts, &availableRaw); err != nil {
		t.Fatal(err)
	}
	if maxAttempts != 2 {
		t.Fatalf("reply max_attempts=%d, want 2", maxAttempts)
	}
	availableAt, err := time.Parse("2006-01-02T15:04:05.000000000Z07:00", availableRaw)
	if err != nil {
		t.Fatal(err)
	}
	delay := availableAt.Sub(before)
	if delay < 9*time.Second || delay > 16*time.Second {
		t.Fatalf("reply backoff delay=%s, want about 10s capped by policy", delay)
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
	a.SetRetryPolicy(5, time.Nanosecond, time.Nanosecond)
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
	if events[0].Phase != "reply" || events[0].ErrorKind != "dependency" || !strings.HasPrefix(events[0].Summary, "reply delivery failed:") {
		t.Fatalf("unexpected runtime event: %#v", events[0])
	}
	if !strings.Contains(events[0].Summary, "550 rcpt") || !strings.Contains(events[0].Summary, "[REDACTED]") {
		t.Fatalf("runtime event should include sanitized SMTP detail: %#v", events[0])
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
