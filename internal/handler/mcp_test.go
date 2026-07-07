package handler

import (
	"context"
	"github.com/liyown/MailRelay/internal/command"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestMCPHTTPCall(t *testing.T) {
	calls := 0
	c := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		body := `{"jsonrpc":"2.0","id":1,"result":{}}`
		if calls == 2 {
			body = `{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"done"}]}}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	h := NewMCP(c)
	res, err := h.Execute(context.Background(), command.Context{Command: command.Command{Name: "tool", Config: map[string]any{"transport": "http", "url": "https://mcp.example.com", "tool": "allowed", "allow_tools": []any{"allowed"}}}, Request: command.Request{Params: map[string]any{"x": "y"}}})
	if err != nil || !strings.Contains(res.Body, "done") {
		t.Fatalf("%#v %v", res, err)
	}
}
