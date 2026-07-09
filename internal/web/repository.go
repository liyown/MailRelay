package web

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/becomeopc/opc-mailrelay/internal/command"
	"github.com/becomeopc/opc-mailrelay/internal/store"
)

type Repository struct {
	store     *store.Store
	mu        sync.RWMutex
	commands  []command.Command
	startedAt time.Time
	now       func() time.Time
}

type ExecutionFilter struct {
	Cursor, Status, Command string
	Limit                   int
}
type JobFilter struct {
	Cursor, Status string
	Limit          int
}
type ReplyFilter struct {
	Cursor, Status string
	Limit          int
}
type EventFilter struct {
	Cursor, Severity string
	Limit            int
}

func NewRepository(s *store.Store, commands []command.Command, startedAt time.Time) *Repository {
	return &Repository{store: s, commands: append([]command.Command(nil), commands...), startedAt: startedAt, now: time.Now}
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func decodeCursor(cursor string) (int64, error) {
	if cursor == "" {
		return 0, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, fmt.Errorf("invalid cursor")
	}
	id, err := strconv.ParseInt(string(raw), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid cursor")
	}
	return id, nil
}

func encodeCursor(id int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(id, 10)))
}

func pageCursor[T any](items []T, limit int, id func(T) int64) string {
	if len(items) < limit || len(items) == 0 {
		return ""
	}
	return encodeCursor(id(items[len(items)-1]))
}

func (r *Repository) Executions(ctx context.Context, filter ExecutionFilter) (Page[ExecutionItem], error) {
	before, err := decodeCursor(filter.Cursor)
	if err != nil {
		return Page[ExecutionItem]{}, err
	}
	limit := clampLimit(filter.Limit)
	rows, err := r.store.ConsoleExecutions(ctx, before, limit, filter.Status, filter.Command)
	if err != nil {
		return Page[ExecutionItem]{}, err
	}
	items := make([]ExecutionItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, ExecutionItem{ID: row.ID, MessageID: row.MessageID, Command: row.Command, Handler: row.Handler, Status: row.Status, Summary: row.Summary, ErrorKind: row.Error, Sender: row.Sender, StartedAt: row.StartedAt, DurationMS: row.Duration.Milliseconds()})
	}
	return Page[ExecutionItem]{Items: items, NextCursor: pageCursor(items, limit, func(v ExecutionItem) int64 { return v.ID })}, nil
}

func (r *Repository) Jobs(ctx context.Context, filter JobFilter) (Page[JobItem], error) {
	before, err := decodeCursor(filter.Cursor)
	if err != nil {
		return Page[JobItem]{}, err
	}
	limit := clampLimit(filter.Limit)
	rows, err := r.store.ConsoleJobs(ctx, before, limit, filter.Status)
	if err != nil {
		return Page[JobItem]{}, err
	}
	items := make([]JobItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, JobItem{ID: row.ID, Command: row.Command, Status: row.Status, Attempts: row.Attempts, MaxAttempts: row.MaxAttempts, AvailableAt: row.AvailableAt})
	}
	return Page[JobItem]{Items: items, NextCursor: pageCursor(items, limit, func(v JobItem) int64 { return v.ID })}, nil
}

func (r *Repository) Replies(ctx context.Context, filter ReplyFilter) (Page[ReplyItem], error) {
	before, err := decodeCursor(filter.Cursor)
	if err != nil {
		return Page[ReplyItem]{}, err
	}
	limit := clampLimit(filter.Limit)
	rows, err := r.store.ConsoleReplies(ctx, before, limit, filter.Status)
	if err != nil {
		return Page[ReplyItem]{}, err
	}
	items := make([]ReplyItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, ReplyItem{ID: row.ID, MessageID: row.MessageID, Recipient: row.Recipient, Status: row.Status, Attempts: row.Attempts, MaxAttempts: row.MaxAttempts, AvailableAt: row.AvailableAt, CreatedAt: row.CreatedAt, ErrorKind: row.LastError})
	}
	return Page[ReplyItem]{Items: items, NextCursor: pageCursor(items, limit, func(v ReplyItem) int64 { return v.ID })}, nil
}

func (r *Repository) Events(ctx context.Context, filter EventFilter) (Page[EventItem], error) {
	before, err := decodeCursor(filter.Cursor)
	if err != nil {
		return Page[EventItem]{}, err
	}
	limit := clampLimit(filter.Limit)
	rows, err := r.store.ConsoleEvents(ctx, before, limit, filter.Severity)
	if err != nil {
		return Page[EventItem]{}, err
	}
	items := make([]EventItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, eventItem(row))
	}
	return Page[EventItem]{Items: items, NextCursor: pageCursor(items, limit, func(v EventItem) int64 { return v.ID })}, nil
}

func (r *Repository) Commands() []CommandItem {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]CommandItem, 0, len(r.commands))
	for _, c := range r.commands {
		items = append(items, CommandItem{Name: c.Name, Description: c.Description, Handler: c.Handler, Maturity: command.HandlerMaturity(c.Handler), ParameterCount: len(c.Parameters)})
	}
	return items
}

// SetCommands atomically swaps the command snapshot the console displays. The
// runtime calls this after every successful config reload or console edit so the
// read views (and command count) reflect the live catalog instead of the
// startup snapshot.
func (r *Repository) SetCommands(commands []command.Command) {
	r.mu.Lock()
	r.commands = append([]command.Command(nil), commands...)
	r.mu.Unlock()
}

func (r *Repository) System() SystemInfo {
	r.mu.RLock()
	count := len(r.commands)
	r.mu.RUnlock()
	return SystemInfo{StartedAt: r.startedAt, UptimeSecond: int64(r.now().Sub(r.startedAt).Seconds()), CommandCount: count}
}

// ReplayJob re-queues a dead queue job so the worker retries it. Idempotent at
// the store level: only rows in the dead state are affected.
func (r *Repository) ReplayJob(ctx context.Context, id int64) error {
	return r.store.ReplayJob(ctx, id)
}

// ReplayReply re-queues a dead SMTP reply for delivery.
func (r *Repository) ReplayReply(ctx context.Context, id int64) error {
	return r.store.ReplayReply(ctx, id)
}

func (r *Repository) Dashboard(ctx context.Context, rangeKey string) (Dashboard, error) {
	duration := map[string]time.Duration{"24h": 24 * time.Hour, "7d": 7 * 24 * time.Hour, "30d": 30 * 24 * time.Hour}[rangeKey]
	if duration == 0 {
		return Dashboard{}, fmt.Errorf("invalid range")
	}
	counts, durations, err := r.store.ConsoleCountsSince(ctx, r.now().Add(-duration))
	if err != nil {
		return Dashboard{}, err
	}
	executions, err := r.Executions(ctx, ExecutionFilter{Limit: 5})
	if err != nil {
		return Dashboard{}, err
	}
	events, err := r.Events(ctx, EventFilter{Limit: 5})
	if err != nil {
		return Dashboard{}, err
	}
	series, err := r.store.ConsoleSeriesSince(ctx, r.now().Add(-duration), 24)
	if err != nil {
		return Dashboard{}, err
	}
	points := make([]SeriesPoint, 0, len(series))
	for _, p := range series {
		points = append(points, SeriesPoint{At: p.BucketStart, Count: p.Count, Success: p.Success})
	}
	result := Dashboard{Range: rangeKey, ExecutionCount: counts.Executions, SuccessCount: counts.Success, ActiveHandlers: counts.ActiveHandlers, Queue: WorkCounts{Pending: counts.QueuePending, Running: counts.QueueRunning, Dead: counts.QueueDead}, Replies: WorkCounts{Pending: counts.ReplyPending, Running: counts.ReplyRunning, Dead: counts.ReplyDead}, Series: points, RecentExecutions: executions.Items, RecentEvents: events.Items}
	if counts.Executions > 0 {
		result.SuccessRate = float64(counts.Success) / float64(counts.Executions) * 100
	}
	if len(durations) > 0 {
		index := (95*len(durations)+99)/100 - 1
		result.P95DurationMS = durations[index].Milliseconds()
	}
	return result, nil
}

func eventItem(row store.RuntimeEvent) EventItem {
	return EventItem{ID: row.ID, At: row.At, Severity: row.Severity, Phase: row.Phase, MessageID: row.MessageID, Command: row.Command, Handler: row.Handler, ErrorKind: row.ErrorKind, Summary: row.Summary}
}
