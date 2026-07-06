package router

import (
	"context"
	"fmt"
	"github.com/becomeopc/opc-mailrelay/internal/command"
	"github.com/becomeopc/opc-mailrelay/internal/handler"
	"sort"
	"strings"
	"time"
)

type Router struct {
	commands map[string]command.Command
	registry *handler.Registry
}

func New(cmds []command.Command, reg *handler.Registry) (*Router, error) {
	r := &Router{commands: map[string]command.Command{}, registry: reg}
	for _, c := range cmds {
		if c.Name == "help" {
			return nil, fmt.Errorf("help is reserved")
		}
		if _, ok := r.commands[c.Name]; ok {
			return nil, fmt.Errorf("duplicate command %s", c.Name)
		}
		if _, ok := reg.Get(c.Handler); !ok {
			return nil, fmt.Errorf("handler %s is not registered", c.Handler)
		}
		r.commands[c.Name] = c
	}
	return r, nil
}
func (r *Router) Execute(ctx context.Context, req command.Request) (res command.Result, err error) {
	started := time.Now()
	defer func() {
		res.StartedAt = started
		res.Duration = time.Since(started)
		if x := recover(); x != nil {
			err = &command.Error{Kind: "internal", Message: "handler panic"}
		}
	}()
	if req.Name == "help" {
		return r.help(req), nil
	}
	c, ok := r.commands[req.Name]
	if !ok {
		return res, &command.Error{Kind: "unknown_command", Message: "unknown command"}
	}
	p, err := command.ValidateParams(c, req.Params)
	if err != nil {
		return res, err
	}
	req.Params = p
	h, _ := r.registry.Get(c.Handler)
	return h.Execute(ctx, command.Context{Command: c, Request: req, Execute: r})
}
func (r *Router) help(req command.Request) command.Result {
	if n, _ := req.Params["command"].(string); n != "" {
		c, ok := r.commands[n]
		if !ok {
			return command.Result{Status: "error", Summary: "Unknown command", Body: "Unknown command"}
		}
		var b strings.Builder
		fmt.Fprintf(&b, "%s\n\n%s\n\nParameters\n", c.Name, c.Description)
		names := make([]string, 0, len(c.Parameters))
		for k := range c.Parameters {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, k := range names {
			p := c.Parameters[k]
			fmt.Fprintf(&b, "- %s: %s", k, p.Description)
			if p.Required {
				b.WriteString(" (required)")
			}
			if p.Example != nil {
				fmt.Fprintf(&b, "; example: %v", p.Example)
			}
			b.WriteByte('\n')
		}
		return command.Result{Status: "success", Summary: "Command help", Body: b.String()}
	}
	names := make([]string, 0, len(r.commands))
	for n := range r.commands {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	b.WriteString("MailRelay Catalog\n\nAvailable Commands\n")
	for _, n := range names {
		fmt.Fprintf(&b, "\n%s - %s", n, r.commands[n].Description)
	}
	return command.Result{Status: "success", Summary: "Available commands", Body: b.String()}
}
func (r *Router) Commands() []command.Command {
	out := make([]command.Command, 0, len(r.commands))
	for _, c := range r.commands {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
func (r *Router) Command(name string) (command.Command, bool) {
	c, ok := r.commands[name]
	return c, ok
}
