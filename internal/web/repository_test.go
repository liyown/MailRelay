package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/becomeopc/opc-mailrelay/internal/command"
	"github.com/becomeopc/opc-mailrelay/internal/store"
)

func seededRepository(t *testing.T) *Repository {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "console.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	base := time.Date(2026, 7, 8, 8, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		_, err = s.AddExecution(ctx, store.Execution{MessageID: fmt.Sprintf("message-%d", i), Command: "notify", Handler: "http", Status: []string{"success", "error", "success"}[i], Summary: "safe summary", Error: "dependency", StartedAt: base.Add(time.Duration(i) * time.Minute), Duration: time.Duration(i+1) * time.Second}, map[string]any{"token": "[redacted]"})
		if err != nil {
			t.Fatal(err)
		}
	}
	_, _ = s.Enqueue(ctx, "notify", map[string]any{"api_key": "topsecret"}, "job-1", 3, base)
	_ = s.AddEvent(ctx, store.RuntimeEvent{At: base, Severity: "error", Phase: "handler", Command: "notify", Handler: "http", ErrorKind: "dependency", Summary: "handler failed"})
	commands := []command.Command{{Name: "notify", Description: "Send notification", Handler: "http", Parameters: map[string]command.Parameter{"message": {Type: "string"}}, Config: map[string]any{"url": "https://example.test/hook", "api_key": "topsecret"}}}
	return NewRepository(s, commands, base.Add(-time.Hour))
}

func TestRepositoryExecutionPaginationIsStable(t *testing.T) {
	repo := seededRepository(t)
	ctx := context.Background()
	first, err := repo.Executions(ctx, ExecutionFilter{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 2 || first.Items[0].ID <= first.Items[1].ID || first.NextCursor == "" {
		t.Fatalf("unexpected first page: %#v", first)
	}
	second, err := repo.Executions(ctx, ExecutionFilter{Limit: 2, Cursor: first.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 1 || second.Items[0].ID >= first.Items[1].ID {
		t.Fatalf("unexpected second page: %#v", second)
	}
}

func TestRepositoryNeverReturnsPersistedSecretFields(t *testing.T) {
	repo := seededRepository(t)
	ctx := context.Background()
	values := []any{}
	page, err := repo.Executions(ctx, ExecutionFilter{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	values = append(values, page, repo.Commands())
	jobs, err := repo.Jobs(ctx, JobFilter{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	values = append(values, jobs)
	raw, _ := json.Marshal(values)
	for _, forbidden := range []string{"topsecret", "api_key", "params_json", "password", "payload"} {
		if bytes.Contains(bytes.ToLower(raw), []byte(forbidden)) {
			t.Fatalf("leaked %s in %s", forbidden, raw)
		}
	}
}

func TestRepositoryDashboardUsesPersistedState(t *testing.T) {
	repo := seededRepository(t)
	dashboard, err := repo.Dashboard(context.Background(), "24h")
	if err != nil {
		t.Fatal(err)
	}
	if dashboard.ExecutionCount != 3 || dashboard.SuccessCount != 2 || len(dashboard.RecentExecutions) == 0 || dashboard.Queue.Pending != 1 {
		t.Fatalf("unexpected dashboard: %#v", dashboard)
	}
}
