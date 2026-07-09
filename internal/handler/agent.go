package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/becomeopc/opc-mailrelay/internal/command"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

type Agent struct{ client *http.Client }

func NewAgent(c *http.Client) *Agent {
	if c == nil {
		c = http.DefaultClient
	}
	return &Agent{c}
}
func (a *Agent) Name() string { return "agent" }
func (a *Agent) Execute(ctx context.Context, x command.Context) (command.Result, error) {
	endpoint, _ := x.Command.Config["endpoint"].(string)
	model, _ := x.Command.Config["model"].(string)
	system, _ := x.Command.Config["system"].(string)
	key, _ := x.Command.Config["api_key"].(string)
	prompt, _ := x.Request.Params["prompt"].(string)
	payload := map[string]any{"model": model, "messages": []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": prompt}}}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(b))
	if err != nil {
		return command.Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	sensitiveValues := append(commandSensitiveValues(x.Command, x.Request.Params), key)
	requestSnapshot := httpRequestSnapshot(req, string(b), sensitiveValues)
	slog.Info("agent request snapshot", "command", x.Command.Name, "message_id", x.Request.MessageID, "request", requestSnapshot["transcript"])
	logHTTPTranscript("AGENT REQUEST", x.Command.Name, x.Request.MessageID, requestSnapshot["transcript"].(string))
	resp, err := a.client.Do(req)
	if err != nil {
		slog.Warn("agent request failed", "command", x.Command.Name, "message_id", x.Request.MessageID, "request", requestSnapshot["transcript"], "error", safeErrorText(err, sensitiveValues))
		return command.Result{}, &command.Error{Kind: "dependency", Message: "agent request failed", Err: err}
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return command.Result{}, err
	}
	responseSnapshot := httpResponseSnapshot(resp, raw, sensitiveValues)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Warn("agent response snapshot", "command", x.Command.Name, "message_id", x.Request.MessageID, "request", requestSnapshot["transcript"], "response", responseSnapshot["transcript"])
		logHTTPTranscript("AGENT RESPONSE", x.Command.Name, x.Request.MessageID, responseSnapshot["transcript"].(string))
		return command.Result{}, &command.Error{Kind: "dependency", Message: fmt.Sprintf("agent status %d: %s", resp.StatusCode, snapshotString(string(raw), sensitiveValues))}
	}
	slog.Info("agent response snapshot", "command", x.Command.Name, "message_id", x.Request.MessageID, "request", requestSnapshot["transcript"], "response", responseSnapshot["transcript"])
	logHTTPTranscript("AGENT RESPONSE", x.Command.Name, x.Request.MessageID, responseSnapshot["transcript"].(string))
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err = json.Unmarshal(raw, &out); err != nil {
		return command.Result{}, err
	}
	if len(out.Choices) == 0 {
		return command.Result{}, fmt.Errorf("agent returned no choices")
	}
	return command.Result{Status: "success", Summary: "Agent completed", Body: strings.TrimSpace(out.Choices[0].Message.Content)}, nil
}
