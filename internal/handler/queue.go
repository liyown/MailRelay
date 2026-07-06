package handler

import (
	"context"
	"fmt"
	"github.com/becomeopc/opc-mailrelay/internal/command"
	"github.com/becomeopc/opc-mailrelay/internal/store"
	"time"
)

type Queue struct{ store *store.Store }

func NewQueue(s *store.Store) *Queue { return &Queue{s} }
func (q *Queue) Name() string        { return "queue" }
func (q *Queue) Execute(ctx context.Context, x command.Context) (command.Result, error) {
	target, _ := x.Command.Config["command"].(string)
	if target == "" || target == x.Command.Name {
		return command.Result{}, fmt.Errorf("invalid queue target")
	}
	max := intValue(x.Command.Config["max_attempts"], 3)
	key := x.Request.MessageID + ":" + x.Command.Name
	id, err := q.store.Enqueue(ctx, target, x.Request.Params, key, max, time.Now())
	if err != nil {
		return command.Result{}, err
	}
	return command.Result{Status: "success", Summary: "Queued", Data: map[string]any{"job_id": id}}, nil
}
func RunOneJob(ctx context.Context, s *store.Store, e command.Executor, lease time.Duration) (bool, error) {
	j, err := s.ClaimJob(ctx, time.Now(), lease)
	if err != nil || j == nil {
		return false, err
	}
	res, err := e.Execute(ctx, command.Request{MessageID: fmt.Sprintf("queue:%d", j.ID), Name: j.Command, Params: j.Params, Received: time.Now()})
	if err != nil {
		if e2 := s.FailJob(ctx, j, err.Error(), time.Duration(j.Attempts)*time.Second); e2 != nil {
			return true, e2
		}
		return true, nil
	}
	return true, s.CompleteJob(ctx, j.ID, res.Summary)
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
