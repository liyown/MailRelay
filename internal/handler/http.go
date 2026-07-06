package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/becomeopc/opc-mailrelay/internal/command"
	"github.com/becomeopc/opc-mailrelay/internal/security"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
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
	raw, _ := x.Command.Config["url"].(string)
	if strings.Contains(raw, "{{") {
		return command.Result{}, &command.Error{Kind: "policy", Message: "URL templates are forbidden"}
	}
	u, err := url.Parse(raw)
	if err != nil {
		return command.Result{}, err
	}
	if err = h.policy.Check(ctx, u); err != nil {
		return command.Result{}, &command.Error{Kind: "policy", Message: "destination denied", Err: err}
	}
	method, _ := x.Command.Config["method"].(string)
	if method == "" {
		method = "POST"
	}
	body, _ := x.Command.Config["body"].(string)
	body = expand(body, x.Request.Params)
	req, err := http.NewRequestWithContext(ctx, method, u.String(), strings.NewReader(body))
	if err != nil {
		return command.Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if hs, ok := x.Command.Config["headers"].(map[string]any); ok {
		for k, v := range hs {
			req.Header.Set(k, fmt.Sprint(v))
		}
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return command.Result{}, &command.Error{Kind: "dependency", Message: "HTTP request failed", Err: err}
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return command.Result{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return command.Result{}, &command.Error{Kind: "dependency", Message: fmt.Sprintf("HTTP status %d", resp.StatusCode)}
	}
	return command.Result{Status: "success", Summary: fmt.Sprintf("HTTP %d", resp.StatusCode), Body: string(b)}, nil
}
func expand(s string, p map[string]any) string {
	for k, v := range p {
		s = strings.ReplaceAll(s, "{{"+k+"}}", fmt.Sprint(v))
	}
	return s
}

type Webhook struct {
	client *http.Client
	policy security.NetworkPolicy
}

func NewWebhook(c *http.Client, p security.NetworkPolicy) *Webhook { return &Webhook{c, p} }
func (w *Webhook) Name() string                                    { return "webhook" }
func (w *Webhook) Execute(ctx context.Context, x command.Context) (command.Result, error) {
	raw, _ := x.Command.Config["url"].(string)
	u, err := url.Parse(raw)
	if err != nil {
		return command.Result{}, err
	}
	if err = w.policy.Check(ctx, u); err != nil {
		return command.Result{}, err
	}
	env := map[string]any{"version": "1", "command": x.Command.Name, "request_id": x.Request.MessageID, "timestamp": time.Now().UTC().Format(time.RFC3339), "params": x.Request.Params}
	b, _ := json.Marshal(env)
	req, err := http.NewRequestWithContext(ctx, "POST", u.String(), bytes.NewReader(b))
	if err != nil {
		return command.Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if secret, _ := x.Command.Config["secret"].(string); secret != "" {
		m := hmac.New(sha256.New, []byte(secret))
		m.Write(b)
		req.Header.Set("X-MailRelay-Signature", "sha256="+hex.EncodeToString(m.Sum(nil)))
	}
	c := w.client
	if c == nil {
		c = w.policy.HTTPClient(30 * time.Second)
	}
	resp, err := c.Do(req)
	if err != nil {
		return command.Result{}, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return command.Result{}, fmt.Errorf("webhook status %d", resp.StatusCode)
	}
	return command.Result{Status: "success", Summary: "Webhook delivered"}, nil
}
