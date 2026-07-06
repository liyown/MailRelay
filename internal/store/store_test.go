package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

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
