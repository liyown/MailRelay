package handler

import (
	"context"
	"fmt"
	"github.com/becomeopc/opc-mailrelay/internal/command"
	"github.com/becomeopc/opc-mailrelay/internal/security"
	"github.com/becomeopc/opc-mailrelay/internal/store"
	"time"
)

type Queue struct {
	store              *store.Store
	defaultMaxAttempts int
	initialBackoff     time.Duration
	maxBackoff         time.Duration
}

func NewQueue(s *store.Store) *Queue {
	return &Queue{store: s, defaultMaxAttempts: 3, initialBackoff: time.Minute, maxBackoff: 30 * time.Minute}
}
func (q *Queue) Name() string { return "queue" }
func (q *Queue) SetRetryPolicy(defaultMaxAttempts int, initialBackoff, maxBackoff time.Duration) {
	if defaultMaxAttempts < 1 {
		defaultMaxAttempts = 1
	}
	if initialBackoff <= 0 {
		initialBackoff = time.Minute
	}
	if maxBackoff <= 0 {
		maxBackoff = initialBackoff
	}
	q.defaultMaxAttempts = defaultMaxAttempts
	q.initialBackoff = initialBackoff
	q.maxBackoff = maxBackoff
}
func (q *Queue) DefaultMaxAttempts() int       { return q.defaultMaxAttempts }
func (q *Queue) InitialBackoff() time.Duration { return q.initialBackoff }
func (q *Queue) MaxBackoff() time.Duration     { return q.maxBackoff }
func (q *Queue) Execute(ctx context.Context, x command.Context) (command.Result, error) {
	target, _ := x.Command.Config["command"].(string)
	if target == "" || target == x.Command.Name {
		return command.Result{}, fmt.Errorf("invalid queue target")
	}
	max := intValue(x.Command.Config["max_attempts"], q.defaultMaxAttempts)
	key := x.Request.MessageID + ":" + x.Command.Name
	redactFrom := x.Command
	if catalog, ok := x.Execute.(command.Catalog); ok {
		if targetCommand, found := catalog.Command(target); found {
			redactFrom = command.MergeSensitiveParameters(x.Command, targetCommand)
		}
	}
	id, err := q.store.Enqueue(ctx, target, security.Redact(redactFrom, x.Request.Params), key, max, time.Now())
	if err != nil {
		return command.Result{}, err
	}
	return command.Result{Status: "success", Summary: "Queued", Data: map[string]any{"job_id": id}}, nil
}
func RunOneJob(ctx context.Context, s *store.Store, e command.Executor, lease time.Duration) (bool, error) {
	return RunOneJobWithPolicy(ctx, s, e, lease, time.Minute, 30*time.Minute)
}
func RunOneJobWithPolicy(ctx context.Context, s *store.Store, e command.Executor, lease, initialBackoff, maxBackoff time.Duration) (bool, error) {
	j, err := s.ClaimJob(ctx, time.Now(), lease)
	if err != nil || j == nil {
		return false, err
	}
	res, err := e.Execute(ctx, command.Request{MessageID: fmt.Sprintf("queue:%d", j.ID), Name: j.Command, Params: j.Params, Received: time.Now()})
	if err != nil {
		if e2 := s.FailJob(ctx, j, err.Error(), retryBackoff(initialBackoff, maxBackoff, j.Attempts)); e2 != nil {
			return true, e2
		}
		return true, nil
	}
	return true, s.CompleteJob(ctx, j.ID, res.Summary)
}
func retryBackoff(initial, max time.Duration, attempt int) time.Duration {
	if initial <= 0 {
		initial = time.Second
	}
	if max <= 0 {
		max = initial
	}
	if attempt < 1 {
		attempt = 1
	}
	backoff := initial
	for i := 1; i < attempt; i++ {
		if backoff >= max/2 {
			backoff = max
			break
		}
		backoff *= 2
	}
	if backoff > max {
		return max
	}
	return backoff
}
func intValue(v any, d int) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	default:
		return d
	}
}
