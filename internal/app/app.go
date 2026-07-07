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
	mu           sync.RWMutex
	store        *store.Store
	router       *router.Router
	sender       mailbox.Sender
	from         string
	allow        []string
	token        string
	replyBackoff time.Duration
}

func runtimeEventSummary(phase string) string {
	switch phase {
	case "reply":
		return "reply delivery failed"
	case "reload":
		return "configuration reload rejected"
	case "receiver":
		return "mail receiver failed"
	default:
		return "runtime failure"
	}
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
		_ = a.store.RecordMessageFailure(ctx, store.MessageUpdate{
			ID:           m.Request.MessageID,
			Sender:       m.Request.Sender,
			State:        store.MessageAuthFailed,
			ErrorKind:    "authentication",
			ErrorSummary: "authentication failed",
		})
		return err
	}
	claimed, err := a.store.ClaimMessage(ctx, m.Request.MessageID, m.Request.Sender)
	if err != nil || !claimed {
		return err
	}
	if err = a.store.MarkMessageExecuting(ctx, m.Request.MessageID, m.Request.Sender, m.Request.Name); err != nil {
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
	if execErr != nil {
		_ = a.store.AddEvent(ctx, store.RuntimeEvent{
			Severity:  "error",
			Phase:     "handler",
			MessageID: m.Request.MessageID,
			Command:   m.Request.Name,
			Handler:   handlerName,
			ErrorKind: safeErr,
			Summary:   "handler failed",
		})
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
		_ = a.store.AddEvent(ctx, store.RuntimeEvent{Severity: "error", Phase: "reply", MessageID: r.MessageID, ErrorKind: "dependency", Summary: runtimeEventSummary("reply")})
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
		_ = a.store.AddEvent(ctx, store.RuntimeEvent{Severity: "error", Phase: "reply", MessageID: r.MessageID, ErrorKind: "dependency", Summary: runtimeEventSummary("reply")})
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

func messageFailureForUID(uid uint32, err error) (store.MessageUpdate, bool) {
	kind := classify(err)
	switch kind {
	case "parse":
		return store.MessageUpdate{
			ID:           fmt.Sprintf("uid:%d", uid),
			State:        store.MessageParseFailed,
			ErrorKind:    "parse",
			ErrorSummary: "parse failed",
		}, true
	case "authentication":
		return store.MessageUpdate{}, false
	default:
		return store.MessageUpdate{
			ID:           fmt.Sprintf("uid:%d", uid),
			State:        store.MessageDead,
			ErrorKind:    "message",
			ErrorSummary: kind,
		}, true
	}
}

func (a *App) Once(ctx context.Context, r mailbox.Receiver, limit int) error {
	msgs, err := r.Fetch(ctx, limit)
	if err != nil {
		return err
	}
	for _, m := range msgs {
		err = a.Process(ctx, mailbox.RawReader(m))
		if err != nil {
			if update, ok := messageFailureForUID(m.UID, err); ok {
				_ = a.store.RecordMessageFailure(ctx, update)
			}
		}
		if markErr := r.MarkSeen(ctx, m.UID); markErr != nil {
			return markErr
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
