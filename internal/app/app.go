package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/liyown/MailRelay/internal/command"
	mailparse "github.com/liyown/MailRelay/internal/mail"
	"github.com/liyown/MailRelay/internal/mailbox"
	"github.com/liyown/MailRelay/internal/router"
	"github.com/liyown/MailRelay/internal/security"
	"github.com/liyown/MailRelay/internal/store"
	"io"
	"sync"
	"time"
)

type App struct {
	mu           sync.RWMutex
	store        *store.Store
	router       *router.Router
	sender       mailbox.Sender
	from         string
	allow        []string
	token        string
	replyBackoff time.Duration
}

func New(s *store.Store, r *router.Router, sender mailbox.Sender, from string, allow []string, token string) *App {
	return &App{store: s, router: r, sender: sender, from: from, allow: allow, token: token, replyBackoff: time.Minute}
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
	reply, err := mailbox.BuildReply(from, m.Request.Sender, m.Request.InReplyTo, m.Request.Name, res)
	if err != nil {
		return err
	}
	handlerName := "builtin"
	if c, ok := route.Command(m.Request.Name); ok {
		handlerName = c.Handler
	}
	id, err := a.store.RecordExecutionAndReply(ctx, store.Execution{MessageID: m.Request.MessageID, Command: m.Request.Name, Handler: handlerName, Status: status, Summary: res.Summary, Error: safeErr, StartedAt: started, Duration: time.Since(started)}, m.Request.Params, m.Request.Sender, reply, 5)
	if err != nil {
		return err
	}
	_, err = a.deliverReply(ctx, id, sender)
	return err
}
func (a *App) deliverReply(ctx context.Context, id int64, sender mailbox.Sender) (bool, error) {
	r, err := a.store.ClaimReplyID(ctx, id, time.Now(), 30*time.Second)
	if err != nil || r == nil {
		return false, err
	}
	if err = sender.Send(ctx, r.Recipient, r.Payload); err != nil {
		if e := a.store.FailReply(ctx, r, err.Error(), a.replyBackoff); e != nil {
			return true, e
		}
		return true, nil
	}
	return true, a.store.CompleteReply(ctx, r)
}
func (a *App) RunOneReply(ctx context.Context) (bool, error) {
	a.mu.RLock()
	sender := a.sender
	a.mu.RUnlock()
	r, err := a.store.ClaimReply(ctx, time.Now(), 30*time.Second)
	if err != nil || r == nil {
		return false, err
	}
	if err = sender.Send(ctx, r.Recipient, r.Payload); err != nil {
		if e := a.store.FailReply(ctx, r, err.Error(), a.replyBackoff); e != nil {
			return true, e
		}
		return true, nil
	}
	return true, a.store.CompleteReply(ctx, r)
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
