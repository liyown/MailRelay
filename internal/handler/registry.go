package handler

import (
	"fmt"
	"github.com/becomeopc/opc-mailrelay/internal/command"
	"sort"
	"sync"
)

type Registry struct {
	mu    sync.RWMutex
	items map[string]command.Handler
}

func NewRegistry() *Registry { return &Registry{items: map[string]command.Handler{}} }
func (r *Registry) Register(h command.Handler) error {
	if h == nil || h.Name() == "" {
		return fmt.Errorf("invalid handler")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[h.Name()]; ok {
		return fmt.Errorf("handler %q already registered", h.Name())
	}
	r.items[h.Name()] = h
	return nil
}
func (r *Registry) Get(name string) (command.Handler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.items[name]
	return h, ok
}
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.items))
	for n := range r.items {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
