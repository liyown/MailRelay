package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/becomeopc/opc-mailrelay/internal/command"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Shell struct{}

func NewShell() *Shell        { return &Shell{} }
func (s *Shell) Name() string { return "shell" }
func (s *Shell) Execute(ctx context.Context, x command.Context) (command.Result, error) {
	exe, args, dir, env, err := processConfig(x)
	if err != nil {
		return command.Result{}, err
	}
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = dir
	cmd.Env = env
	var out bytes.Buffer
	cmd.Stdout = &limitedWriter{w: &out, n: 1 << 20}
	cmd.Stderr = &limitedWriter{w: &out, n: 1 << 20}
	if err = cmd.Run(); err != nil {
		sensitiveValues := commandSensitiveValues(x.Command, x.Request.Params)
		detail := safeLogText(out.String(), sensitiveValues)
		slog.Warn("shell process failed", "command", x.Command.Name, "message_id", x.Request.MessageID, "executable", exe, "args", safeStringSlice(args, sensitiveValues), "output", detail, "error", safeErrorText(err, nil))
		return command.Result{}, &command.Error{Kind: "dependency", Message: "process failed", Err: fmt.Errorf("%w: %s", err, detail)}
	}
	sensitiveValues := commandSensitiveValues(x.Command, x.Request.Params)
	slog.Info("shell process completed", "command", x.Command.Name, "message_id", x.Request.MessageID, "executable", exe, "args", safeStringSlice(args, sensitiveValues), "output", safeLogText(out.String(), sensitiveValues))
	return command.Result{Status: "success", Summary: "Process completed", Body: out.String()}, nil
}

type Plugin struct{}

func NewPlugin() *Plugin       { return &Plugin{} }
func (p *Plugin) Name() string { return "plugin" }
func (p *Plugin) Execute(ctx context.Context, x command.Context) (command.Result, error) {
	exe, args, dir, env, err := processConfig(x)
	if err != nil {
		return command.Result{}, err
	}
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = dir
	cmd.Env = env
	input, _ := json.Marshal(map[string]any{"version": "1", "command": x.Command.Name, "params": x.Request.Params, "request_id": x.Request.MessageID})
	cmd.Stdin = bytes.NewReader(input)
	var out bytes.Buffer
	cmd.Stdout = &limitedWriter{w: &out, n: 1 << 20}
	var stderr bytes.Buffer
	cmd.Stderr = &limitedWriter{w: &stderr, n: 64 << 10}
	if err = cmd.Run(); err != nil {
		sensitiveValues := commandSensitiveValues(x.Command, x.Request.Params)
		detail := safeLogText(stderr.String(), sensitiveValues)
		slog.Warn("plugin process failed", "command", x.Command.Name, "message_id", x.Request.MessageID, "executable", exe, "args", safeStringSlice(args, sensitiveValues), "stderr", detail, "error", safeErrorText(err, nil))
		return command.Result{}, &command.Error{Kind: "dependency", Message: "plugin failed", Err: fmt.Errorf("%w: %s", err, detail)}
	}
	var res command.Result
	if err = json.Unmarshal(out.Bytes(), &res); err != nil {
		sensitiveValues := commandSensitiveValues(x.Command, x.Request.Params)
		slog.Warn("plugin returned invalid JSON", "command", x.Command.Name, "message_id", x.Request.MessageID, "executable", exe, "stdout", safeLogText(out.String(), sensitiveValues), "stderr", safeLogText(stderr.String(), sensitiveValues), "error", safeErrorText(err, nil))
		return command.Result{}, &command.Error{Kind: "dependency", Message: "invalid plugin result", Err: err}
	}
	if res.Status == "" {
		res.Status = "success"
	}
	sensitiveValues := commandSensitiveValues(x.Command, x.Request.Params)
	slog.Info("plugin completed", "command", x.Command.Name, "message_id", x.Request.MessageID, "executable", exe, "status", res.Status, "summary", safeLogText(res.Summary, sensitiveValues))
	return res, nil
}
func processConfig(x command.Context) (string, []string, string, []string, error) {
	exe, _ := x.Command.Config["executable"].(string)
	if !filepath.IsAbs(exe) {
		return "", nil, "", nil, fmt.Errorf("executable must be absolute")
	}
	st, err := os.Stat(exe)
	if err != nil {
		return "", nil, "", nil, err
	}
	if !st.Mode().IsRegular() || st.Mode().Perm()&0002 != 0 {
		return "", nil, "", nil, fmt.Errorf("unsafe executable")
	}
	var args []string
	if a, ok := x.Command.Config["args"].([]any); ok {
		for _, v := range a {
			args = append(args, expand(fmt.Sprint(v), x.Request.Params))
		}
	}
	dir, _ := x.Command.Config["working_dir"].(string)
	if dir == "" {
		dir = "/"
	}
	env := []string{"PATH=/usr/bin:/bin"}
	if e, ok := x.Command.Config["env"].(map[string]any); ok {
		for k, v := range e {
			if strings.ContainsAny(k, "=\x00") {
				return "", nil, "", nil, fmt.Errorf("invalid environment key")
			}
			env = append(env, k+"="+expand(fmt.Sprint(v), x.Request.Params))
		}
	}
	return exe, args, dir, env, nil
}

type limitedWriter struct {
	w io.Writer
	n int64
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if l.n <= 0 {
		return len(p), nil
	}
	q := p
	if int64(len(q)) > l.n {
		q = q[:l.n]
	}
	_, err := l.w.Write(q)
	l.n -= int64(len(q))
	return len(p), err
}
