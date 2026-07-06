package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/becomeopc/opc-mailrelay/internal/command"
	mailparse "github.com/becomeopc/opc-mailrelay/internal/mail"
	"github.com/becomeopc/opc-mailrelay/internal/mailbox"
	"github.com/becomeopc/opc-mailrelay/internal/router"
	"github.com/becomeopc/opc-mailrelay/internal/security"
	"github.com/becomeopc/opc-mailrelay/internal/store"
	"io"
	"sync"
	"time"
)

type App struct {
	mu     sync.RWMutex
	store  *store.Store
	router *router.Router
	sender mailbox.Sender
	from   string
	allow  []string
	token  string
}

func New(s *store.Store, r *router.Router, sender mailbox.Sender, from string, allow []string, token string) *App {
	return &App{store: s, router: r, sender: sender, from: from, allow: allow, token: token}
}
func (a *App) Process(ctx context.Context, raw io.ReadCloser) error {
	defer raw.Close()
	m, err := mailparse.Parse(raw)
	if err != nil {
		return err
	}
	a.mu.RLock()
	allow := append([]string(nil), a.allow...)
	token, from, sender, route := a.token, a.from, a.sender, a.router
	a.mu.RUnlock()
	if err = security.Authenticate(m.Request, m.Token, allow, token); err != nil {
		return err
	}
	claimed, err := a.store.ClaimMessage(ctx, m.Request.MessageID, m.Request.Sender)
	if err != nil || !claimed {
		return err
	}
	started := time.Now()
	res, execErr := route.Execute(ctx, m.Request)
	status := "success"
	safeErr := ""
	if execErr != nil {
		status = "error"
		safeErr = classify(execErr)
		res = command.Result{Status: "error", Summary: safeErr, Body: "The command could not be completed."}
	}
	if res.Status == "" {
		res.Status = status
	}
	_, auditErr := a.store.AddExecution(ctx, store.Execution{MessageID: m.Request.MessageID, Command: m.Request.Name, Status: status, Summary: res.Summary, Error: safeErr, StartedAt: started, Duration: time.Since(started)}, m.Request.Params)
	if auditErr != nil {
		return auditErr
	}
	reply, err := mailbox.BuildReply(from, m.Request.Sender, m.Request.InReplyTo, m.Request.Name, res)
	if err != nil {
		return err
	}
	if err = sender.Send(ctx, m.Request.Sender, reply); err != nil {
		_ = a.store.MarkMessage(ctx, m.Request.MessageID, status, true)
		return err
	}
	return a.store.MarkMessage(ctx, m.Request.MessageID, status, false)
}
func (a *App) ReplaceRouter(r *router.Router) { a.mu.Lock(); a.router = r; a.mu.Unlock() }
func (a *App) ReplaceSecurity(from string, allow []string, token string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.from = from
	a.allow = append([]string(nil), allow...)
	a.token = token
}
func (a *App) Execute(ctx context.Context, req command.Request) (command.Result, error) {
	a.mu.RLock()
	r := a.router
	a.mu.RUnlock()
	return r.Execute(ctx, req)
}
func classify(err error) string {
	var e *command.Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return "internal"
}
func (a *App) Once(ctx context.Context, r mailbox.Receiver, limit int) error {
	msgs, err := r.Fetch(ctx, limit)
	if err != nil {
		return err
	}
	for _, m := range msgs {
		if err = a.Process(ctx, mailbox.RawReader(m)); err != nil {
			return fmt.Errorf("message %d: %w", m.UID, err)
		}
		if err = r.MarkSeen(ctx, m.UID); err != nil {
			return err
		}
	}
	return nil
}
func IgnoreNoRows(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return err
}
