package handler

import (
	"context"
	"github.com/becomeopc/opc-mailrelay/internal/command"
	"os"
	"testing"
)

func TestShellDoesNotInterpretMetacharacters(t *testing.T) {
	h := NewShell()
	res, err := h.Execute(context.Background(), command.Context{Command: command.Command{Name: "echo", Config: map[string]any{"executable": "/bin/echo", "args": []any{"{{value}}"}}}, Request: command.Request{Params: map[string]any{"value": "hello; exit 7"}}})
	if err != nil || res.Body != "hello; exit 7\n" {
		t.Fatalf("%q %v", res.Body, err)
	}
}
func TestPluginJSONProtocol(t *testing.T) {
	if _, err := os.Stat("/usr/bin/python3"); err != nil {
		t.Skip("python unavailable")
	}
	h := NewPlugin()
	c := command.Command{Name: "p", Config: map[string]any{"executable": "/usr/bin/python3", "args": []any{"-c", "import sys,json; x=json.load(sys.stdin); print(json.dumps({'status':'success','summary':x['command']}))"}}}
	res, err := h.Execute(context.Background(), command.Context{Command: c, Request: command.Request{Params: map[string]any{"x": "y"}}})
	if err != nil || res.Summary != "p" {
		t.Fatalf("%#v %v", res, err)
	}
}
