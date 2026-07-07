package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/becomeopc/opc-mailrelay/internal/catalog"
	"github.com/becomeopc/opc-mailrelay/internal/command"
	"github.com/becomeopc/opc-mailrelay/internal/config"
	"github.com/becomeopc/opc-mailrelay/internal/handler"
	"github.com/becomeopc/opc-mailrelay/internal/mailbox"
	"github.com/becomeopc/opc-mailrelay/internal/router"
	"github.com/becomeopc/opc-mailrelay/internal/security"
	"github.com/becomeopc/opc-mailrelay/internal/store"
)

type Runtime struct {
	mu       sync.RWMutex
	path     string
	cfg      *config.Config
	store    *store.Store
	app      *App
	receiver mailbox.Receiver
	sender   mailbox.Sender
	registry *handler.Registry
	custom   []command.Handler
	mtime    time.Time
}

func Build(path string, custom ...command.Handler) (*Runtime, error) {
	cfg, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	s, err := store.Open(cfg.Storage.Path)
	if err != nil {
		return nil, err
	}
	sender := mailbox.NewSMTP(cfg.Mail.SMTP)
	receiver := mailbox.NewIMAP(cfg.Mail.IMAP)
	reg, route, err := buildRouter(cfg, s, custom)
	if err != nil {
		s.Close()
		return nil, err
	}
	r := &Runtime{path: path, cfg: cfg, store: s, receiver: receiver, sender: sender, registry: reg, custom: custom}
	r.app = New(s, route, sender, cfg.Mail.SMTP.From, cfg.Security.Allow, cfg.Security.Token)
	if st, e := os.Stat(path); e == nil {
		r.mtime = st.ModTime()
	}
	if err = r.updateCatalog(context.Background(), nil, cfg.Commands); err != nil {
		s.Close()
		return nil, err
	}
	return r, nil
}

func buildRouter(cfg *config.Config, s *store.Store, custom []command.Handler) (*handler.Registry, *router.Router, error) {
	reg := handler.NewRegistry()
	policy := security.NetworkPolicy{Hosts: cfg.Security.HTTPHosts}
	client := policy.HTTPClient(30 * time.Second)
	builtins := []command.Handler{handler.NewHTTP(client, policy), handler.NewWebhook(client, policy), handler.NewWorkflow(64, 8), handler.NewPlugin(), handler.NewShell(), handler.NewAgent(client), handler.NewMCP(client), handler.NewQueue(s)}
	for _, h := range append(builtins, custom...) {
		if err := reg.Register(h); err != nil {
			return nil, nil, err
		}
	}
	r, err := router.New(cfg.Commands, reg)
	if err == nil {
		if d, e := time.ParseDuration(cfg.Runtime.CommandTimeout); e == nil && d > 0 {
			r.SetTimeout(d)
		}
	}
	return reg, r, err
}
func (r *Runtime) Close() error        { return r.store.Close() }
func (r *Runtime) Store() *store.Store { return r.store }
func (r *Runtime) Once(ctx context.Context) error {
	r.mu.RLock()
	a := r.app
	recv := r.receiver
	r.mu.RUnlock()
	if err := a.Once(ctx, recv, 100); err != nil {
		return err
	}
	for i := 0; i < 100; i++ {
		worked, err := a.RunOneReply(ctx)
		if err != nil {
			return err
		}
		if !worked {
			break
		}
	}
	for i := 0; i < 100; i++ {
		worked, err := handler.RunOneJob(ctx, r.store, a, 30*time.Second)
		if err != nil {
			return err
		}
		if !worked {
			break
		}
	}
	_ = r.store.SetState(ctx, "last_poll", time.Now().UTC().Format(time.RFC3339))
	_ = r.store.SetState(ctx, "last_runtime_error", "")
	return nil
}
func (r *Runtime) Run(ctx context.Context) error {
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return nil
		}
		if err := r.reloadIfChanged(ctx); err != nil {
			_ = r.store.AddEvent(ctx, store.RuntimeEvent{Severity: "error", Phase: "reload", ErrorKind: "config", Summary: err.Error()})
			slog.Error("configuration reload rejected", "error", err)
		}
		if err := r.Once(ctx); err != nil {
			_ = r.store.AddEvent(context.Background(), store.RuntimeEvent{Severity: "error", Phase: "receiver", ErrorKind: "dependency", Summary: err.Error()})
			_ = r.store.SetState(context.Background(), "last_runtime_error", err.Error())
			slog.Error("mail poll failed", "error", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(backoff):
			}
			if backoff < time.Minute {
				backoff *= 2
			}
			continue
		}
		backoff = time.Second
		r.mu.RLock()
		recv := r.receiver
		wait := r.cfg.Mail.IMAP.PollInterval
		r.mu.RUnlock()
		idleCtx, cancel := context.WithTimeout(ctx, wait)
		err := recv.Idle(idleCtx)
		cancel()
		if ctx.Err() != nil {
			return nil
		}
		if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
			slog.Warn("IMAP IDLE unavailable; polling fallback active", "error", err)
		}
	}
}
func (r *Runtime) reloadIfChanged(ctx context.Context) error {
	st, err := os.Stat(r.path)
	if err != nil {
		return err
	}
	if !st.ModTime().After(r.mtime) {
		return nil
	}
	cfg, err := config.Load(r.path)
	if err != nil {
		_ = r.store.AddEvent(ctx, store.RuntimeEvent{Severity: "error", Phase: "reload", ErrorKind: "config", Summary: err.Error()})
		return err
	}
	reg, route, err := buildRouter(cfg, r.store, r.custom)
	if err != nil {
		_ = r.store.AddEvent(ctx, store.RuntimeEvent{Severity: "error", Phase: "reload", ErrorKind: "config", Summary: err.Error()})
		return err
	}
	r.mu.RLock()
	old := r.cfg
	r.mu.RUnlock()
	if err = r.updateCatalog(ctx, old.Commands, cfg.Commands); err != nil {
		_ = r.store.AddEvent(ctx, store.RuntimeEvent{Severity: "error", Phase: "reload", ErrorKind: "config", Summary: err.Error()})
		return err
	}
	r.mu.Lock()
	r.cfg = cfg
	r.registry = reg
	r.receiver = mailbox.NewIMAP(cfg.Mail.IMAP)
	r.sender = mailbox.NewSMTP(cfg.Mail.SMTP)
	r.app.mu.Lock()
	r.app.sender = r.sender
	r.app.mu.Unlock()
	r.app.ReplaceRouter(route)
	r.app.ReplaceSecurity(cfg.Mail.SMTP.From, cfg.Security.Allow, cfg.Security.Token)
	r.mtime = st.ModTime()
	r.mu.Unlock()
	return nil
}
func (r *Runtime) updateCatalog(ctx context.Context, oldHint, newCmds []command.Command) error {
	raw, hash := catalog.Build(newCmds)
	oldHash, oldRaw, _, err := r.store.Catalog(ctx)
	if err != nil && !errors.Is(err, os.ErrNotExist) && !isNoRows(err) {
		return err
	}
	if oldHash == "" {
		return r.store.SaveCatalog(ctx, hash, raw, true)
	}
	if oldHash == hash {
		return nil
	}
	var old []command.Command
	if len(oldRaw) > 0 {
		_ = json.Unmarshal(oldRaw, &old)
	} else {
		old = oldHint
	}
	diff := catalog.Diff(old, newCmds)
	r.mu.RLock()
	notify := append([]string(nil), r.cfg.Runtime.CatalogNotify...)
	sender := r.sender
	r.mu.RUnlock()
	if len(notify) > 0 && diff != "" {
		if err = sender.Notify(ctx, notify, "MailRelay Catalog Changed", diff); err != nil {
			_ = r.store.SaveCatalog(ctx, hash, raw, false)
			return fmt.Errorf("catalog notification: %w", err)
		}
	}
	return r.store.SaveCatalog(ctx, hash, raw, true)
}
func isNoRows(err error) bool { return err != nil && err.Error() == "sql: no rows in result set" }
