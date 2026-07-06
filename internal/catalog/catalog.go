package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/becomeopc/opc-mailrelay/internal/command"
	"sort"
	"strings"
)

func Build(cmds []command.Command) ([]byte, string) {
	x := append([]command.Command(nil), cmds...)
	sort.Slice(x, func(i, j int) bool { return x[i].Name < x[j].Name })
	type catalogCommand struct {
		command.Command
		Maturity string `json:"maturity"`
	}
	items := make([]catalogCommand, 0, len(x))
	for _, c := range x {
		items = append(items, catalogCommand{Command: c, Maturity: command.HandlerMaturity(c.Handler)})
	}
	b, _ := json.Marshal(items)
	s := sha256.Sum256(b)
	return b, hex.EncodeToString(s[:])
}
func Diff(old, new []command.Command) string {
	om := map[string]string{}
	nm := map[string]string{}
	for _, c := range old {
		b, _ := json.Marshal(c)
		om[c.Name] = string(b)
	}
	for _, c := range new {
		b, _ := json.Marshal(c)
		nm[c.Name] = string(b)
	}
	var add, rem, upd []string
	for n, v := range nm {
		if ov, ok := om[n]; !ok {
			add = append(add, n)
		} else if ov != v {
			upd = append(upd, n)
		}
	}
	for n := range om {
		if _, ok := nm[n]; !ok {
			rem = append(rem, n)
		}
	}
	sort.Strings(add)
	sort.Strings(rem)
	sort.Strings(upd)
	var b strings.Builder
	section := func(title string, x []string) {
		if len(x) == 0 {
			return
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(title)
		for _, n := range x {
			fmt.Fprintf(&b, "\n- %s", n)
		}
	}
	section("Added", add)
	section("Removed", rem)
	section("Updated", upd)
	return b.String()
}
