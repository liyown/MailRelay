package handler

import (
	"context"
	"fmt"
	"github.com/becomeopc/opc-mailrelay/internal/command"
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
	summaries := make([]any, 0, len(raw))
	for _, v := range raw {
		step, ok := v.(map[string]any)
		if !ok {
			return command.Result{}, fmt.Errorf("invalid workflow step")
		}
		name, _ := step["command"].(string)
		if name == "" || name == x.Command.Name {
			return command.Result{}, fmt.Errorf("workflow recursion denied")
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
		res, err := x.Execute.Execute(ctx, command.Request{MessageID: x.Request.MessageID + ":" + name, Sender: x.Request.Sender, Name: name, Params: params, Received: x.Request.Received})
		if err != nil {
			return command.Result{}, err
		}
		summaries = append(summaries, map[string]any{"command": name, "status": res.Status, "summary": res.Summary})
	}
	return command.Result{Status: "success", Summary: "Workflow completed", Data: map[string]any{"steps": summaries}}, nil
}
