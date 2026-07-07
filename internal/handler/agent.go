package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/liyown/MailRelay/internal/command"
	"io"
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
	resp, err := a.client.Do(req)
	if err != nil {
		return command.Result{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return command.Result{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return command.Result{}, fmt.Errorf("agent status %d", resp.StatusCode)
	}
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
