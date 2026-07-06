package handler

import (
	"context"
	"github.com/becomeopc/opc-mailrelay/internal/command"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAgentUsesFixedModel(t *testing.T) {
	var got string
	c := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(r.Body)
		got = string(b)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"answer"}}]}`)), Header: make(http.Header)}, nil
	})}
	h := NewAgent(c)
	res, err := h.Execute(context.Background(), command.Context{Command: command.Command{Name: "ask", Config: map[string]any{"endpoint": "https://ai.example.com/v1/chat/completions", "model": "fixed", "api_key": "secret", "system": "safe"}}, Request: command.Request{Params: map[string]any{"prompt": "hi"}}})
	if err != nil || res.Body != "answer" || !strings.Contains(got, `"model":"fixed"`) {
		t.Fatalf("%s %#v %v", got, res, err)
	}
}
