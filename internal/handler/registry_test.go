package handler

import (
	"context"
	"github.com/becomeopc/opc-mailrelay/internal/command"
	"testing"
)

type fake struct{ name string }

func (f fake) Name() string { return f.name }
func (f fake) Execute(context.Context, command.Context) (command.Result, error) {
	return command.Result{Status: "ok"}, nil
}
func TestRegistry(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(fake{"x"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Get("x"); !ok {
		t.Fatal("missing")
	}
	if err := r.Register(fake{"x"}); err == nil {
		t.Fatal("expected duplicate")
	}
}
