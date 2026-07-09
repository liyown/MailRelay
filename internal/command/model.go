package command

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

type Parameter struct {
	Description string `yaml:"description" json:"description"`
	Type        string `yaml:"type" json:"type"`
	Required    bool   `yaml:"required" json:"required"`
	Sensitive   bool   `yaml:"sensitive" json:"sensitive"`
	Example     any    `yaml:"example" json:"example,omitempty"`
}

type Command struct {
	Name        string               `yaml:"name" json:"name"`
	Description string               `yaml:"description" json:"description"`
	Handler     string               `yaml:"handler" json:"handler"`
	Parameters  map[string]Parameter `yaml:"parameters" json:"parameters,omitempty"`
	Config      map[string]any       `yaml:"config" json:"config,omitempty"`
}

type Request struct {
	MessageID string
	Sender    string
	Name      string
	Params    map[string]any
	RawBody   string
	Received  time.Time
	InReplyTo string
	Trace     []string
}

type Context struct {
	Command Command
	Request Request
	Execute Executor
}
type Executor interface {
	Execute(context.Context, Request) (Result, error)
}

type Catalog interface {
	Command(string) (Command, bool)
}

func MergeSensitiveParameters(commands ...Command) Command {
	merged := Command{Parameters: map[string]Parameter{}}
	for _, c := range commands {
		for name, p := range c.Parameters {
			existing := merged.Parameters[name]
			if p.Sensitive {
				existing.Sensitive = true
			}
			merged.Parameters[name] = existing
		}
	}
	return merged
}

type Result struct {
	Status, Summary, Body string
	Data                  map[string]any
	StartedAt             time.Time
	Duration              time.Duration
}
type Handler interface {
	Name() string
	Execute(context.Context, Context) (Result, error)
}

func HandlerMaturity(name string) string {
	switch name {
	case "http", "http_request", "webhook", "workflow", "queue":
		return "Stable"
	default:
		return "Experimental"
	}
}

type Error struct {
	Kind, Message string
	Err           error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Kind + ": " + e.Message + ": " + e.Err.Error()
	}
	return e.Kind + ": " + e.Message
}
func (e *Error) Unwrap() error { return e.Err }

func ValidateParams(c Command, in map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(in))
	for k := range in {
		if _, ok := c.Parameters[k]; !ok {
			return nil, &Error{Kind: "invalid_parameters", Message: "unknown parameter " + k}
		}
	}
	for name, p := range c.Parameters {
		v, ok := in[name]
		if !ok {
			if p.Required {
				return nil, &Error{Kind: "invalid_parameters", Message: "missing parameter " + name}
			}
			continue
		}
		var err error
		switch p.Type {
		case "", "string":
			if _, ok := v.(string); !ok {
				err = fmt.Errorf("must be string")
			}
		case "integer":
			switch x := v.(type) {
			case int:
				v = x
			case int64:
				v = x
			case float64:
				if x != float64(int64(x)) {
					err = fmt.Errorf("must be integer")
				} else {
					v = int64(x)
				}
			case string:
				v, err = strconv.ParseInt(x, 10, 64)
			default:
				err = fmt.Errorf("must be integer")
			}
		case "number":
			if _, ok := v.(float64); !ok {
				err = fmt.Errorf("must be number")
			}
		case "boolean":
			if _, ok := v.(bool); !ok {
				err = fmt.Errorf("must be boolean")
			}
		default:
			err = fmt.Errorf("unsupported type %s", p.Type)
		}
		if err != nil {
			return nil, &Error{Kind: "invalid_parameters", Message: name, Err: err}
		}
		out[name] = v
	}
	return out, nil
}
