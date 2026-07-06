package handler

import (
	"context"
	"github.com/becomeopc/opc-mailrelay/internal/command"
	"github.com/becomeopc/opc-mailrelay/internal/store"
	"path/filepath"
	"testing"
	"time"
)

func TestQueueAndWorker(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	h := NewQueue(s)
	_, err = h.Execute(context.Background(), command.Context{Command: command.Command{Name: "later", Config: map[string]any{"command": "deploy", "max_attempts": 2}}, Request: command.Request{MessageID: "m1", Params: map[string]any{"env": "prod"}}})
	if err != nil {
		t.Fatal(err)
	}
	e := &execCapture{}
	worked, err := RunOneJob(context.Background(), s, e, time.Second)
	if err != nil || !worked || len(e.names) != 1 || e.names[0] != "deploy" {
		t.Fatalf("%v %#v %v", worked, e.names, err)
	}
}
