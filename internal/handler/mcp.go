package handler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/becomeopc/opc-mailrelay/internal/command"
	"io"
	"net/http"
	"os/exec"
	"strings"
)

type MCP struct{ client *http.Client }

func NewMCP(c *http.Client) *MCP {
	if c == nil {
		c = http.DefaultClient
	}
	return &MCP{c}
}
func (m *MCP) Name() string { return "mcp" }
func (m *MCP) Execute(ctx context.Context, x command.Context) (command.Result, error) {
	tool, _ := x.Command.Config["tool"].(string)
	if !allowedTool(tool, x.Command.Config["allow_tools"]) {
		return command.Result{}, &command.Error{Kind: "policy", Message: "MCP tool is not allowed"}
	}
	transport, _ := x.Command.Config["transport"].(string)
	var raw json.RawMessage
	var err error
	if transport == "stdio" {
		raw, err = m.callStdio(ctx, x, tool)
	} else {
		raw, err = m.callHTTP(ctx, x, tool)
	}
	if err != nil {
		return command.Result{}, err
	}
	var out struct {
		Content []struct{ Type, Text string } `json:"content"`
	}
	if err = json.Unmarshal(raw, &out); err != nil {
		return command.Result{}, err
	}
	var parts []string
	for _, c := range out.Content {
		if c.Type == "text" {
			parts = append(parts, c.Text)
		}
	}
	return command.Result{Status: "success", Summary: "MCP tool completed", Body: strings.Join(parts, "\n")}, nil
}
func allowedTool(tool string, v any) bool {
	if a, ok := v.([]any); ok {
		for _, x := range a {
			if fmt.Sprint(x) == tool {
				return true
			}
		}
	}
	return false
}
func (m *MCP) callHTTP(ctx context.Context, x command.Context, tool string) (json.RawMessage, error) {
	url, _ := x.Command.Config["url"].(string)
	if _, err := m.rpcHTTP(ctx, url, 1, "initialize", map[string]any{"protocolVersion": "2025-03-26", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "mailrelay", "version": "1"}}); err != nil {
		return nil, err
	}
	return m.rpcHTTP(ctx, url, 2, "tools/call", map[string]any{"name": tool, "arguments": x.Request.Params})
}
func (m *MCP) rpcHTTP(ctx context.Context, url string, id int, method string, params any) (json.RawMessage, error) {
	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	var msg struct {
		Result json.RawMessage `json:"result"`
		Error  any             `json:"error"`
	}
	if err = json.Unmarshal(raw, &msg); err != nil {
		return nil, err
	}
	if msg.Error != nil {
		return nil, fmt.Errorf("MCP error: %v", msg.Error)
	}
	return msg.Result, nil
}
func (m *MCP) callStdio(ctx context.Context, x command.Context, tool string) (json.RawMessage, error) {
	exe, _ := x.Command.Config["executable"].(string)
	var args []string
	if a, ok := x.Command.Config["args"].([]any); ok {
		for _, v := range a {
			args = append(args, fmt.Sprint(v))
		}
	}
	cmd := exec.CommandContext(ctx, exe, args...)
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	enc := json.NewEncoder(in)
	dec := json.NewDecoder(bufio.NewReader(io.LimitReader(out, 4<<20)))
	var msg struct {
		Result json.RawMessage `json:"result"`
		Error  any             `json:"error"`
	}
	if err = enc.Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"protocolVersion": "2025-03-26", "capabilities": map[string]any{}}}); err == nil {
		err = dec.Decode(&msg)
	}
	if err == nil {
		err = enc.Encode(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/call", "params": map[string]any{"name": tool, "arguments": x.Request.Params}})
	}
	if err == nil {
		err = dec.Decode(&msg)
	}
	in.Close()
	waitErr := cmd.Wait()
	if err != nil {
		return nil, err
	}
	if waitErr != nil {
		return nil, waitErr
	}
	if msg.Error != nil {
		return nil, fmt.Errorf("MCP error: %v", msg.Error)
	}
	return msg.Result, nil
}
