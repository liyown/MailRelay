package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestReplyOutboxLeaseRetryAndDeadLetter(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "outbox.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	id, err := s.RecordExecutionAndReply(ctx, Execution{MessageID: "m1", Command: "push", Handler: "http", Status: "success", StartedAt: time.Now()}, nil, "me@example.com", []byte("reply"), 2)
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.ClaimReply(ctx, time.Now(), time.Minute)
	if err != nil || r == nil || r.ID != id {
		t.Fatalf("%#v %v", r, err)
	}
	if err = s.FailReply(ctx, r, "550 rcpt <me@example.com> rejected", 0); err != nil {
		t.Fatal(err)
	}
	r, err = s.ClaimReply(ctx, time.Now().Add(time.Second), time.Minute)
	if err != nil || r == nil {
		t.Fatalf("%#v %v", r, err)
	}
	if err = s.FailReply(ctx, r, "535 auth token=secret still down", 0); err != nil {
		t.Fatal(err)
	}
	state, err := s.MessageState(ctx, "m1")
	if err != nil {
		t.Fatal(err)
	}
	if state.State != MessageDead || state.ErrorKind != "reply_delivery" || state.ErrorSummary != "delivery failed" {
		t.Fatalf("unexpected message state after dead reply: %#v", state)
	}
	failure, err := s.LatestFailure(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if failure != "reply: delivery failed" {
		t.Fatalf("unexpected latest failure: %q", failure)
	}
	pending, dead, err := s.ReplyCounts(ctx)
	if err != nil || pending != 0 || dead != 1 {
		t.Fatalf("pending=%d dead=%d err=%v", pending, dead, err)
	}
	if err = s.ReplayReply(ctx, id); err != nil {
		t.Fatal(err)
	}
	pending, dead, err = s.ReplyCounts(ctx)
	if err != nil || pending != 1 || dead != 0 {
		t.Fatalf("pending=%d dead=%d err=%v", pending, dead, err)
	}
}

func TestClaimsPersistAndQueue(t *testing.T) {
	p := filepath.Join(t.TempDir(), "relay.db")
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ok, err := s.ClaimMessage(context.Background(), "m1", "me@example.com")
	if err != nil || !ok {
		t.Fatalf("%v %v", ok, err)
	}
	ok, err = s.ClaimMessage(context.Background(), "m1", "me@example.com")
	if err != nil || ok {
		t.Fatalf("duplicate claimed: %v %v", ok, err)
	}
	id, err := s.Enqueue(context.Background(), "deploy", map[string]any{"x": "y"}, "key", 2, time.Now())
	if err != nil || id == 0 {
		t.Fatal(err)
	}
	if _, err = s.Enqueue(context.Background(), "deploy", nil, "key", 2, time.Now()); err != nil {
		t.Fatal(err)
	}
	j, err := s.ClaimJob(context.Background(), time.Now(), time.Minute)
	if err != nil || j == nil || j.Command != "deploy" {
		t.Fatalf("%#v %v", j, err)
	}
	if err = s.CompleteJob(context.Background(), j.ID, "ok"); err != nil {
		t.Fatal(err)
	}
}

func TestMessageLifecycleStatesArePersisted(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "messages.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err := s.RecordMessageFailure(ctx, MessageUpdate{ID: "parse:1", Sender: "bad@example.com", State: MessageParseFailed, ErrorKind: "parse", ErrorSummary: "missing subject"}); err != nil {
		t.Fatal(err)
	}
	got, err := s.MessageState(ctx, "parse:1")
	if err != nil {
		t.Fatal(err)
	}
	if got.State != MessageParseFailed || got.ErrorKind != "parse" || got.ErrorSummary != "missing subject" {
		t.Fatalf("unexpected state: %#v", got)
	}
	if err := s.MarkMessageExecuting(ctx, "m1", "me@example.com", "push"); err != nil {
		t.Fatal(err)
	}
	got, err = s.MessageState(ctx, "m1")
	if err != nil {
		t.Fatal(err)
	}
	if got.State != MessageExecuting || got.Command != "push" || got.Sender != "me@example.com" {
		t.Fatalf("unexpected executing state: %#v", got)
	}
}

func TestMessageFailureUpdatePreservesExistingSenderAndCommand(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "preserve.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err := s.MarkMessageExecuting(ctx, "m1", "me@example.com", "push"); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordMessageFailure(ctx, MessageUpdate{
		ID:           "m1",
		State:        MessageDead,
		ErrorKind:    "reply_delivery",
		ErrorSummary: "smtp unavailable",
	}); err != nil {
		t.Fatal(err)
	}
	got, err := s.MessageState(ctx, "m1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Sender != "me@example.com" || got.Command != "push" || got.State != MessageDead {
		t.Fatalf("unexpected preserved state: %#v", got)
	}
}

func TestQueueClaimsSameSecondFractionalTimesChronologically(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "fractional.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	available := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	now := time.Date(2026, 7, 7, 12, 0, 0, 1, time.UTC)
	if _, err := s.Enqueue(ctx, "deploy", nil, "fractional-key", 1, available); err != nil {
		t.Fatal(err)
	}
	j, err := s.ClaimJob(ctx, now, time.Minute)
	if err != nil || j == nil {
		t.Fatalf("claim at %s for available %s returned %#v, %v", now.Format(time.RFC3339Nano), available.Format(time.RFC3339Nano), j, err)
	}
}

func TestCatalogAndRuntime(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err = s.SaveCatalog(ctx, "hash", []byte(`[]`), true); err != nil {
		t.Fatal(err)
	}
	h, _, notified, err := s.Catalog(ctx)
	if err != nil || h != "hash" || !notified {
		t.Fatalf("%s %v %v", h, notified, err)
	}
	if err = s.SetState(ctx, "last_poll", "now"); err != nil {
		t.Fatal(err)
	}
	v, err := s.State(ctx, "last_poll")
	if err != nil || v != "now" {
		t.Fatalf("%s %v", v, err)
	}
}

func TestQueueDeadLetterReplayAndLatestFailure(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "dead.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	id, err := s.Enqueue(ctx, "deploy", nil, "dead-key", 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	j, err := s.ClaimJob(ctx, time.Now(), time.Minute)
	if err != nil || j == nil {
		t.Fatalf("%#v %v", j, err)
	}
	if err = s.FailJob(ctx, j, "boom", 0); err != nil {
		t.Fatal(err)
	}
	qd, rd, err := s.DeadCounts(ctx)
	if err != nil || qd != 1 || rd != 0 {
		t.Fatalf("queue=%d reply=%d err=%v", qd, rd, err)
	}
	failure, err := s.LatestFailure(ctx)
	if err != nil || failure != "queue: boom" {
		t.Fatalf("%q %v", failure, err)
	}
	if err = s.ReplayJob(ctx, id); err != nil {
		t.Fatal(err)
	}
	depth, err := s.QueueDepth(ctx)
	if err != nil || depth != 1 {
		t.Fatalf("depth=%d err=%v", depth, err)
	}
}

func TestRuntimeEventsAndHealthSummary(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "health.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err := s.AddEvent(ctx, RuntimeEvent{Severity: "error", Phase: "reload", ErrorKind: "config", Summary: "invalid yaml"}); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkMessageExecuting(ctx, "stale-1", "me@example.com", "push"); err != nil {
		t.Fatal(err)
	}
	events, err := s.RecentEvents(ctx, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Phase != "reload" || events[0].Summary != "invalid yaml" {
		t.Fatalf("events=%#v", events)
	}
	health, err := s.Health(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if health.StaleExecuting != 1 || len(health.LatestFailures) != 1 {
		t.Fatalf("health=%#v", health)
	}
}
