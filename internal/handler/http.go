package handler

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/becomeopc/opc-mailrelay/internal/command"
	"github.com/becomeopc/opc-mailrelay/internal/security"
)

type HTTP struct {
	client *http.Client
	policy security.NetworkPolicy
}

func NewHTTP(c *http.Client, p security.NetworkPolicy) *HTTP {
	if c == nil {
		c = p.HTTPClient(30 * time.Second)
	}
	return &HTTP{c, p}
}

func (h *HTTP) Name() string { return "http" }

func (h *HTTP) Execute(ctx context.Context, x command.Context) (command.Result, error) {
	req, body, err := h.buildRequest(ctx, x)
	if err != nil {
		return command.Result{}, err
	}
	sensitiveValues := commandSensitiveValues(x.Command, x.Request.Params)
	requestSnapshot := httpRequestSnapshot(req, body, sensitiveValues)
	logHTTPSnapshot("http request snapshot", "HTTP REQUEST", x, requestSnapshot["transcript"].(string))

	resp, err := h.client.Do(req)
	if err != nil {
		slog.Warn("http request failed", "command", x.Command.Name, "message_id", x.Request.MessageID, "request", requestSnapshot["transcript"], "error", redactSnapshotText(err.Error(), sensitiveValues))
		return command.Result{}, &command.Error{Kind: "dependency", Message: "HTTP request failed", Err: err}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return command.Result{}, err
	}
	responseSnapshot := httpResponseSnapshot(resp, respBody, sensitiveValues)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logHTTPSnapshot("http response snapshot", "HTTP RESPONSE", x, requestSnapshot["transcript"].(string), responseSnapshot["transcript"].(string))
		return command.Result{}, &command.Error{Kind: "dependency", Message: fmt.Sprintf("HTTP status %d: %s", resp.StatusCode, snapshotString(string(respBody), sensitiveValues))}
	}
	logHTTPSnapshot("http response snapshot", "HTTP RESPONSE", x, requestSnapshot["transcript"].(string), responseSnapshot["transcript"].(string))
	return command.Result{Status: "success", Summary: fmt.Sprintf("HTTP %d", resp.StatusCode), Body: string(respBody)}, nil
}

func (h *HTTP) buildRequest(ctx context.Context, x command.Context) (*http.Request, string, error) {
	raw, _ := x.Command.Config["url"].(string)
	u, err := parseTemplatedURL(raw, x.Request.Params)
	if err != nil {
		return nil, "", err
	}
	if err = h.policy.Check(ctx, u); err != nil {
		return nil, "", &command.Error{Kind: "policy", Message: "destination denied", Err: err}
	}
	applyQuery(u, x.Command.Config["query"], x.Request.Params)

	method, _ := x.Command.Config["method"].(string)
	if method == "" {
		method = "POST"
	}
	body, _ := x.Command.Config["body"].(string)
	body = expand(body, x.Request.Params)

	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return nil, "", err
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if hs, ok := x.Command.Config["headers"].(map[string]any); ok {
		for k, v := range hs {
			req.Header.Set(k, fmt.Sprint(v))
		}
	}
	return req, body, nil
}
