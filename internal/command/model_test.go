package command

import "testing"

func TestValidateParams(t *testing.T) {
	c := Command{Name: "push", Parameters: map[string]Parameter{"message": {Type: "string", Required: true}, "count": {Type: "integer"}}}
	p, err := ValidateParams(c, map[string]any{"message": "hi", "count": 2})
	if err != nil || p["message"] != "hi" {
		t.Fatalf("%v %#v", err, p)
	}
	if _, err := ValidateParams(c, map[string]any{}); err == nil {
		t.Fatal("expected missing error")
	}
	if _, err := ValidateParams(c, map[string]any{"message": "hi", "extra": true}); err == nil {
		t.Fatal("expected unknown error")
	}
}

func TestHandlerMaturityStableCore(t *testing.T) {
	for _, name := range []string{"http", "webhook", "workflow", "queue"} {
		if got := HandlerMaturity(name); got != "Stable" {
			t.Fatalf("HandlerMaturity(%q)=%q, want Stable", name, got)
		}
	}
	for _, name := range []string{"plugin", "shell", "agent", "mcp"} {
		if got := HandlerMaturity(name); got != "Experimental" {
			t.Fatalf("HandlerMaturity(%q)=%q, want Experimental", name, got)
		}
	}
}
