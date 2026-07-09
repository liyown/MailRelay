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
	var gotURL string
	var gotContentType string
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotURL = r.URL.String()
		gotContentType = r.Header.Get("Content-Type")
		if r.Body != nil {
			body, _ = io.ReadAll(r.Body)
		} else {
			body = nil
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
	})}
	policy := security.NetworkPolicy{Hosts: []string{"api.example.com"}, LookupIP: func(context.Context, string) ([]net.IP, error) { return []net.IP{net.ParseIP("93.184.216.34")}, nil }}
	h := NewHTTP(client, policy)
	res, err := h.Execute(context.Background(), command.Context{Command: command.Command{Name: "push", Handler: "http", Config: map[string]any{"url": "https://api.example.com/push", "method": "POST", "body": "{\"message\":\"{{message}}\"}"}}, Request: command.Request{Params: map[string]any{"message": "hello"}}})
	if err != nil || res.Status != "success" || !strings.Contains(string(body), "hello") {
		t.Fatalf("%s %#v %v", body, res, err)
	}
	_, err = h.Execute(context.Background(), command.Context{
		Command: command.Command{Name: "lookup", Handler: "http", Config: map[string]any{
			"url":    "https://api.example.com/search?fixed=1",
			"method": "GET",
			"query":  map[string]any{"q": "{{term}}", "page": "{{page}}"},
		}},
		Request: command.Request{Params: map[string]any{"term": "hello world", "page": int64(2)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotURL != "https://api.example.com/search?fixed=1&page=2&q=hello+world" {
		t.Fatalf("url query not expanded/appended: %s", gotURL)
	}
	if gotContentType != "" {
		t.Fatalf("GET without body should not send Content-Type, got %q", gotContentType)
	}
	_, err = h.Execute(context.Background(), command.Context{
		Command: command.Command{Name: "path", Handler: "http", Config: map[string]any{
			"url":    "https://api.example.com/users/{{user_id}}/notify",
			"method": "POST",
		}},
		Request: command.Request{Params: map[string]any{"user_id": "team/a"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotURL != "https://api.example.com/users/team%2Fa/notify" {
		t.Fatalf("url path not expanded/escaped: %s", gotURL)
	}
	_, err = h.Execute(context.Background(), command.Context{
		Command: command.Command{Name: "bad-host", Handler: "http", Config: map[string]any{
			"url": "https://{{host}}/notify",
		}},
		Request: command.Request{Params: map[string]any{"host": "api.example.com"}},
	})
	if err == nil {
		t.Fatal("expected static host rejection")
	}
	w := NewWebhook(client, policy)
	_, err = w.Execute(context.Background(), command.Context{Command: command.Command{Name: "hook", Config: map[string]any{"url": "https://api.example.com/hook/{{x}}", "secret": "key", "query": map[string]any{"source": "{{x}}"}}}, Request: command.Request{MessageID: "m1", Params: map[string]any{"x": "y"}}})
	if err != nil {
		t.Fatal(err)
	}
	if gotURL != "https://api.example.com/hook/y?source=y" {
		t.Fatalf("webhook url query not expanded/appended: %s", gotURL)
	}
	mac := hmac.New(sha256.New, []byte("key"))
	mac.Write(body)
	if hex.EncodeToString(mac.Sum(nil)) == "" {
		t.Fatal("missing signature")
	}
}

func TestHTTPSnapshotRedactsSensitiveRequestAndResponse(t *testing.T) {
	reqHeader := http.Header{}
	reqHeader.Set("Authorization", "Bearer topsecret")
	reqHeader.Set("X-Trace", "user-secret-value")
	req, err := http.NewRequest(http.MethodPost, "https://api.example.com/users/user-secret-value?token=user-secret-value", strings.NewReader("password=user-secret-value"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header = reqHeader
	sensitive := []string{"user-secret-value"}

	request := httpRequestSnapshot(req, "password=user-secret-value", sensitive)
	if got := request["url"].(string); strings.Contains(got, "user-secret-value") {
		t.Fatalf("sensitive url value leaked: %s", got)
	}
	if got := request["transcript"].(string); !strings.Contains(got, "POST /users/[REDACTED]?token=[REDACTED] HTTP/1.1") || !strings.Contains(got, "Host: api.example.com") {
		t.Fatalf("request transcript is not HTTP-like:\n%s", got)
	}
	headers := request["headers"].(map[string][]string)
	if got := strings.Join(headers["Authorization"], ","); got != "[REDACTED]" {
		t.Fatalf("authorization header not redacted: %q", got)
	}
	if got := strings.Join(headers["X-Trace"], ","); strings.Contains(got, "user-secret-value") {
		t.Fatalf("sensitive header value leaked: %q", got)
	}
	if got := request["body"].(string); strings.Contains(got, "user-secret-value") {
		t.Fatalf("sensitive body value leaked: %q", got)
	}

	resp := &http.Response{StatusCode: 500, Status: "500 Internal Server Error", Header: http.Header{"Set-Cookie": []string{"sid=secret"}, "X-Error": []string{"bad user-secret-value"}}}
	response := httpResponseSnapshot(resp, []byte("failed for user-secret-value"), sensitive)
	if got := response["transcript"].(string); !strings.Contains(got, "HTTP/1.1 500 Internal Server Error") || !strings.Contains(got, "failed for [REDACTED]") {
		t.Fatalf("response transcript is not HTTP-like:\n%s", got)
	}
	respHeaders := response["headers"].(map[string][]string)
	if got := strings.Join(respHeaders["Set-Cookie"], ","); got != "[REDACTED]" {
		t.Fatalf("cookie header not redacted: %q", got)
	}
	if got := response["body"].(string); strings.Contains(got, "user-secret-value") {
		t.Fatalf("sensitive response body leaked: %q", got)
	}
}

func TestHTTPRequestForwardsRawMailBody(t *testing.T) {
	var gotMethod, gotURL, gotAccept string
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotMethod = r.Method
		gotURL = r.URL.String()
		gotAccept = r.Header.Get("Accept")
		return &http.Response{StatusCode: 200, Status: "200 OK", Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
	})}
	policy := security.NetworkPolicy{Hosts: []string{"api.example.com"}, LookupIP: func(context.Context, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("93.184.216.34")}, nil
	}}
	h := NewHTTPRequest(client, policy)
	res, err := h.Execute(context.Background(), command.Context{
		Command: command.Command{Name: "forward", Handler: "http_request", Config: map[string]any{"base_url": "https://api.example.com"}},
		Request: command.Request{
			MessageID: "raw-1",
			RawBody:   "GET /push/hello?x=1 HTTP/1.1\r\nHost: api.example.com\r\nAccept: application/json\r\n\r\n",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "success" || gotMethod != "GET" || gotURL != "https://api.example.com/push/hello?x=1" || gotAccept != "application/json" {
		t.Fatalf("res=%#v method=%s url=%s accept=%s", res, gotMethod, gotURL, gotAccept)
	}
}
