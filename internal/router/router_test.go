package router

import (
	"context"
	"github.com/becomeopc/opc-mailrelay/internal/command"
	"github.com/becomeopc/opc-mailrelay/internal/handler"
	"strings"
	"testing"
)

type capture struct{ got command.Context }

func (c *capture) Name() string { return "capture" }
func (c *capture) Execute(_ context.Context, x command.Context) (command.Result, error) {
	c.got = x
	return command.Result{Status: "success", Summary: "done"}, nil
}
func TestRouteAndHelp(t *testing.T) {
	h := &capture{}
	reg := handler.NewRegistry()
	_ = reg.Register(h)
	cmd := command.Command{Name: "deploy", Description: "Deploy app", Handler: "capture", Parameters: map[string]command.Parameter{"env": {Description: "Environment", Type: "string", Required: true, Example: "prod"}}}
	r, err := New([]command.Command{cmd}, reg)
	if err != nil {
		t.Fatal(err)
	}
	res, err := r.Execute(context.Background(), command.Request{Name: "deploy", Params: map[string]any{"env": "prod"}})
	if err != nil || h.got.Request.Params["env"] != "prod" {
		t.Fatalf("%#v %v", res, err)
	}
	res, err = r.Execute(context.Background(), command.Request{Name: "help", Params: map[string]any{"command": "deploy"}})
	if err != nil || !strings.Contains(res.Body, "Deploy app") || !strings.Contains(res.Body, "Environment") || !strings.Contains(res.Body, "prod") {
		t.Fatalf("%s %v", res.Body, err)
	}
}
