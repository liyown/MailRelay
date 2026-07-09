package handler

import (
	"context"
	"errors"
	"fmt"
	"github.com/becomeopc/opc-mailrelay/internal/command"
	"log/slog"
)

type Workflow struct{ maxSteps, maxDepth int }

func NewWorkflow(steps, depth int) *Workflow { return &Workflow{steps, depth} }
func (w *Workflow) Name() string             { return "workflow" }
func (w *Workflow) Execute(ctx context.Context, x command.Context) (command.Result, error) {
	if x.Execute == nil {
		return command.Result{}, fmt.Errorf("workflow executor is unavailable")
	}
	raw, ok := x.Command.Config["steps"].([]any)
	if !ok || len(raw) == 0 {
		return command.Result{}, fmt.Errorf("workflow has no steps")
	}
	if len(raw) > w.maxSteps {
		return command.Result{}, fmt.Errorf("workflow exceeds maximum steps")
	}
	if len(x.Request.Trace)+1 >= w.maxDepth {
		return command.Result{}, &command.Error{Kind: "policy", Message: "workflow depth exceeded"}
	}
	summaries := make([]any, 0, len(raw))
	for i, v := range raw {
		step, ok := v.(map[string]any)
		if !ok {
			return command.Result{}, fmt.Errorf("invalid workflow step")
		}
		name, _ := step["command"].(string)
		if name == "" {
			return command.Result{}, &command.Error{Kind: "policy", Message: "workflow step has no command"}
		}
		if name == x.Command.Name {
			return command.Result{}, &command.Error{Kind: "policy", Message: "workflow recursion denied"}
		}
		params := map[string]any{}
		if p, ok := step["params"].(map[string]any); ok {
			for k, v := range p {
				if s, ok := v.(string); ok {
					v = expand(s, x.Request.Params)
				}
				params[k] = v
			}
		}
		trace := append([]string(nil), x.Request.Trace...)
		trace = append(trace, x.Command.Name)
		stepMessageID := fmt.Sprintf("%s:step:%d:%s", x.Request.MessageID, i+1, name)
		slog.Info("workflow step started", "command", x.Command.Name, "message_id", x.Request.MessageID, "step", i+1, "target", name, "step_message_id", stepMessageID)
		res, err := x.Execute.Execute(ctx, command.Request{
			MessageID: stepMessageID,
			Sender:    x.Request.Sender,
			Name:      name,
			Params:    params,
			Received:  x.Request.Received,
			Trace:     trace,
		})
		if err != nil {
			var commandErr *command.Error
			if errors.As(err, &commandErr) && commandErr.Kind == "policy" {
				return command.Result{}, err
			}
			slog.Warn("workflow step failed", "command", x.Command.Name, "message_id", x.Request.MessageID, "step", i+1, "target", name, "error", safeErrorText(err, commandSensitiveValues(x.Command, x.Request.Params)))
			return command.Result{}, &command.Error{
				Kind:    "workflow_step",
				Message: fmt.Sprintf("step %d (%s) failed", i+1, name),
				Err:     err,
			}
		}
		slog.Info("workflow step completed", "command", x.Command.Name, "message_id", x.Request.MessageID, "step", i+1, "target", name, "status", res.Status, "summary", safeLogText(res.Summary, commandSensitiveValues(x.Command, x.Request.Params)))
		summaries = append(summaries, map[string]any{"command": name, "status": res.Status, "summary": res.Summary})
	}
	return command.Result{Status: "success", Summary: "Workflow completed", Data: map[string]any{"steps": summaries}}, nil
}
