package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"github.com/becomeopc/opc-mailrelay/internal/command"
	"github.com/becomeopc/opc-mailrelay/internal/security"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestHTTPAndWebhook(t *testing.T) {
	var body []byte
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ = io.ReadAll(r.Body)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
	})}
	policy := security.NetworkPolicy{Hosts: []string{"api.example.com"}, LookupIP: func(context.Context, string) ([]net.IP, error) { return []net.IP{net.ParseIP("93.184.216.34")}, nil }}
	h := NewHTTP(client, policy)
	res, err := h.Execute(context.Background(), command.Context{Command: command.Command{Name: "push", Handler: "http", Config: map[string]any{"url": "https://api.example.com/push", "method": "POST", "body": "{\"message\":\"{{message}}\"}"}}, Request: command.Request{Params: map[string]any{"message": "hello"}}})
	if err != nil || res.Status != "success" || !strings.Contains(string(body), "hello") {
		t.Fatalf("%s %#v %v", body, res, err)
	}
	w := NewWebhook(client, policy)
	_, err = w.Execute(context.Background(), command.Context{Command: command.Command{Name: "hook", Config: map[string]any{"url": "https://api.example.com/hook", "secret": "key"}}, Request: command.Request{MessageID: "m1", Params: map[string]any{"x": "y"}}})
	if err != nil {
		t.Fatal(err)
	}
	mac := hmac.New(sha256.New, []byte("key"))
	mac.Write(body)
	if hex.EncodeToString(mac.Sum(nil)) == "" {
		t.Fatal("missing signature")
	}
}
