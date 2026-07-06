package handler

import (
	"context"
	"github.com/becomeopc/opc-mailrelay/internal/command"
	"testing"
)

type execCapture struct{ names []string }

func (e *execCapture) Execute(_ context.Context, r command.Request) (command.Result, error) {
	e.names = append(e.names, r.Name)
	return command.Result{Status: "success", Data: map[string]any{"ok": true}}, nil
}
func TestWorkflow(t *testing.T) {
	e := &execCapture{}
	h := NewWorkflow(10, 4)
	c := command.Command{Name: "release", Config: map[string]any{"steps": []any{map[string]any{"command": "build"}, map[string]any{"command": "deploy", "params": map[string]any{"env": "{{env}}"}}}}}
	res, err := h.Execute(context.Background(), command.Context{Command: c, Request: command.Request{Params: map[string]any{"env": "prod"}}, Execute: e})
	if err != nil || res.Status != "success" || len(e.names) != 2 {
		t.Fatalf("%#v %v", res, err)
	}
}
