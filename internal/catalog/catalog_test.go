package catalog

import (
	"github.com/becomeopc/opc-mailrelay/internal/command"
	"strings"
	"testing"
)

func TestStableAndDiff(t *testing.T) {
	a := []command.Command{{Name: "b", Description: "B", Handler: "http"}, {Name: "a", Description: "A", Handler: "http"}}
	b := []command.Command{{Name: "a", Description: "updated", Handler: "http"}, {Name: "c", Description: "C", Handler: "http"}}
	_, ha := Build(a)
	_, ha2 := Build([]command.Command{a[1], a[0]})
	if ha != ha2 {
		t.Fatal("unstable")
	}
	d := Diff(a, b)
	if !strings.Contains(d, "Added\n- c") || !strings.Contains(d, "Removed\n- b") || !strings.Contains(d, "Updated\n- a") {
		t.Fatal(d)
	}
}
