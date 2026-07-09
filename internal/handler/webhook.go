package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/becomeopc/opc-mailrelay/internal/command"
	"github.com/becomeopc/opc-mailrelay/internal/security"
)

type Webhook struct {
	client *http.Client
	policy security.NetworkPolicy
}

func NewWebhook(c *http.Client, p security.NetworkPolicy) *Webhook { return &Webhook{c, p} }

func (w *Webhook) Name() string { return "webhook" }

func (w *Webhook) Execute(ctx context.Context, x command.Context) (command.Result, error) {
	req, body, err := w.buildRequest(ctx, x)
	if err != nil {
		return command.Result{}, err
	}
	sensitiveValues := commandSensitiveValues(x.Command, x.Request.Params)
	requestSnapshot := httpRequestSnapshot(req, string(body), sensitiveValues)
	logHTTPSnapshot("webhook request snapshot", "WEBHOOK REQUEST", x, requestSnapshot["transcript"].(string))

	c := w.client
	if c == nil {
		c = w.policy.HTTPClient(30 * time.Second)
	}
	resp, err := c.Do(req)
	if err != nil {
		slog.Warn("webhook request failed", "command", x.Command.Name, "message_id", x.Request.MessageID, "request", requestSnapshot["transcript"], "error", redactSnapshotText(err.Error(), sensitiveValues))
		return command.Result{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return command.Result{}, err
	}
	responseSnapshot := httpResponseSnapshot(resp, respBody, sensitiveValues)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logHTTPSnapshot("webhook response snapshot", "WEBHOOK RESPONSE", x, requestSnapshot["transcript"].(string), responseSnapshot["transcript"].(string))
		return command.Result{}, &command.Error{Kind: "dependency", Message: fmt.Sprintf("webhook status %d: %s", resp.StatusCode, snapshotString(string(respBody), sensitiveValues))}
	}
	logHTTPSnapshot("webhook response snapshot", "WEBHOOK RESPONSE", x, requestSnapshot["transcript"].(string), responseSnapshot["transcript"].(string))
	return command.Result{Status: "success", Summary: fmt.Sprintf("Webhook delivered: HTTP %d", resp.StatusCode), Body: string(respBody)}, nil
}

func (w *Webhook) buildRequest(ctx context.Context, x command.Context) (*http.Request, []byte, error) {
	raw, _ := x.Command.Config["url"].(string)
	u, err := url.Parse(raw)
	if err != nil {
		return nil, nil, err
	}
	if err = prepareURL(u, x.Request.Params); err != nil {
		return nil, nil, err
	}
	if err = w.policy.Check(ctx, u); err != nil {
		return nil, nil, err
	}
	applyQuery(u, x.Command.Config["query"], x.Request.Params)

	env := map[string]any{"version": "1", "command": x.Command.Name, "request_id": x.Request.MessageID, "timestamp": time.Now().UTC().Format(time.RFC3339), "params": x.Request.Params}
	body, _ := json.Marshal(env)
	req, err := http.NewRequestWithContext(ctx, "POST", u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if secret, _ := x.Command.Config["secret"].(string); secret != "" {
		m := hmac.New(sha256.New, []byte(secret))
		m.Write(body)
		req.Header.Set("X-MailRelay-Signature", "sha256="+hex.EncodeToString(m.Sum(nil)))
	}
	return req, body, nil
}
