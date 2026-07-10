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
	"log/slog"
	mailstd "net/mail"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type App struct {
	mu               sync.RWMutex
	store            *store.Store
	router           *router.Router
	sender           mailbox.Sender
	from             string
	allow            []string
	token            string
	replyMaxAttempts int
	initialBackoff   time.Duration
	maxBackoff       time.Duration
}

// MailPreview is the result of parsing and validating a mail command without
// claiming a message, executing a handler, or sending a reply.
type MailPreview struct {
	Accepted   bool
	Stage      string
	Command    string
	Handler    string
	Parameters []string
	ErrorKind  string
}

var (
	emailAddressRE = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	secretValueRE  = regexp.MustCompile(`(?i)\b(token|password|passwd|secret|authorization|bearer)\b\s*[:=]?\s*[^;,\s"']+`)
)

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

func redactErrorDetail(v string) string {
	v = strings.NewReplacer("\r", " ", "\n", " ", "\t", " ").Replace(v)
	v = strings.Join(strings.Fields(v), " ")
	v = emailAddressRE.ReplaceAllString(v, "[email]")
	v = secretValueRE.ReplaceAllString(v, "$1=[REDACTED]")
	if len(v) > 240 {
		v = v[:240] + "..."
	}
	return v
}

func safeErrorDetail(err error) string {
	if err == nil {
		return ""
	}
	var e *command.Error
	if errors.As(err, &e) {
		detail := e.Message
		if e.Err != nil {
			detail += ": " + e.Err.Error()
		}
		return redactErrorDetail(detail)
	}
	return redactErrorDetail(err.Error())
}

func summaryWithDetail(summary string, err error) string {
	detail := safeErrorDetail(err)
	if detail == "" {
		return summary
	}
	return summary + ": " + detail
}

func safeAuthReason(err error) string {
	var e *command.Error
	if !errors.As(err, &e) {
		return "authentication failed"
	}
	switch e.Message {
	case "invalid token":
		return "invalid token"
	case "sender is not allowed":
		return "sender not allowed"
	default:
		return "authentication failed"
	}
}

func sameMailboxAddress(a, b string) bool {
	addrA, errA := mailstd.ParseAddress(a)
	if errA == nil {
		a = addrA.Address
	}
	addrB, errB := mailstd.ParseAddress(b)
	if errB == nil {
		b = addrB.Address
	}
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func New(s *store.Store, r *router.Router, sender mailbox.Sender, from string, allow []string, token string) *App {
	return &App{store: s, router: r, sender: sender, from: from, allow: allow, token: token, replyMaxAttempts: 5, initialBackoff: time.Minute, maxBackoff: 30 * time.Minute}
}
func (a *App) SetRetryPolicy(replyMaxAttempts int, initialBackoff, maxBackoff time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if replyMaxAttempts < 1 {
		replyMaxAttempts = 1
	}
	if initialBackoff <= 0 {
		initialBackoff = time.Minute
	}
	if maxBackoff <= 0 {
		maxBackoff = initialBackoff
	}
	a.replyMaxAttempts = replyMaxAttempts
	a.initialBackoff = initialBackoff
	a.maxBackoff = maxBackoff
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
	if sameMailboxAddress(m.Request.Sender, from) {
		if recordErr := a.store.RecordMessageFailure(ctx, store.MessageUpdate{
			ID:           m.Request.MessageID,
			Sender:       m.Request.Sender,
			State:        store.MessageDead,
			Command:      m.Request.Name,
			ErrorKind:    "self_message",
			ErrorSummary: "self-generated message ignored",
		}); recordErr != nil {
			return recordErr
		}
		_ = a.store.AddEvent(ctx, store.RuntimeEvent{
			Severity:  "warn",
			Phase:     "message",
			MessageID: m.Request.MessageID,
			Command:   m.Request.Name,
			ErrorKind: "self_message",
			Summary:   "self-generated message ignored",
		})
		slog.Warn("self-generated mail message ignored", "message_id", m.Request.MessageID, "command", m.Request.Name)
		return nil
	}
	if err = security.Authenticate(m.Request, m.Token, allow, token); err != nil {
		reason := safeAuthReason(err)
		if recordErr := a.store.RecordMessageFailure(ctx, store.MessageUpdate{
			ID:           m.Request.MessageID,
			Sender:       m.Request.Sender,
			State:        store.MessageAuthFailed,
			Command:      m.Request.Name,
			ErrorKind:    "authentication",
			ErrorSummary: reason,
		}); recordErr != nil {
			return recordErr
		}
		if _, execErr := a.store.AddExecution(ctx, store.Execution{
			MessageID: m.Request.MessageID,
			Command:   m.Request.Name,
			Handler:   "auth",
			Status:    "error",
			Summary:   reason,
			Error:     "authentication",
			StartedAt: time.Now(),
		}, m.Request.Params); execErr != nil {
			return execErr
		}
		_ = a.store.AddEvent(ctx, store.RuntimeEvent{
			Severity:  "warn",
			Phase:     "auth",
			MessageID: m.Request.MessageID,
			Command:   m.Request.Name,
			ErrorKind: "authentication",
			Summary:   reason,
		})
		slog.Warn("mail command rejected", "phase", "auth", "reason", reason, "message_id", m.Request.MessageID, "command", m.Request.Name)
		a.sendFailureReply(ctx, sender, from, m.Request.Sender, m.Request.InReplyTo, m.Request.Name, "authentication", reason)
		return err
	}
	claimed, err := a.store.ClaimMessage(ctx, m.Request.MessageID, m.Request.Sender)
	if err != nil || !claimed {
		if err == nil && !claimed {
			_ = a.store.AddEvent(ctx, store.RuntimeEvent{
				Severity:  "info",
				Phase:     "message",
				MessageID: m.Request.MessageID,
				Command:   m.Request.Name,
				Summary:   "duplicate message ignored",
			})
			slog.Info("duplicate mail command ignored", "message_id", m.Request.MessageID, "command", m.Request.Name)
		}
		return err
	}
	if err = a.store.MarkMessageExecuting(ctx, m.Request.MessageID, m.Request.Sender, m.Request.Name); err != nil {
		return err
	}
	started := time.Now()
	res, execErr := route.Execute(ctx, m.Request)
	status := "success"
	safeErr := ""
	execSummary := ""
	if execErr != nil {
		status = "error"
		safeErr = classify(execErr)
		execSummary = summaryWithDetail(safeErr, execErr)
		res = command.Result{Status: "error", Summary: execSummary, Body: "The command could not be completed."}
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
			Summary:   summaryWithDetail("handler failed", execErr),
		})
		slog.Error("mail command handler failed", "message_id", m.Request.MessageID, "command", m.Request.Name, "handler", handlerName, "error_kind", safeErr, "error", safeErrorDetail(execErr), "error_type", fmt.Sprintf("%T", execErr))
	}
	persistedParams := m.Request.Params
	if c, ok := route.Command(m.Request.Name); ok {
		redactFrom := c
		if target, _ := c.Config["command"].(string); target != "" {
			if targetCommand, found := route.Command(target); found {
				redactFrom = command.MergeSensitiveParameters(c, targetCommand)
			}
		}
		persistedParams = security.Redact(redactFrom, m.Request.Params)
	}
	a.mu.RLock()
	replyMaxAttempts := a.replyMaxAttempts
	a.mu.RUnlock()
	if execSummary != "" {
		res.Summary = execSummary
	}
	id, err := a.store.RecordExecutionAndReply(ctx, store.Execution{MessageID: m.Request.MessageID, Command: m.Request.Name, Handler: handlerName, Status: status, Summary: res.Summary, Error: safeErr, StartedAt: started, Duration: time.Since(started)}, persistedParams, m.Request.Sender, reply, replyMaxAttempts)
	if err != nil {
		return err
	}
	_, err = a.deliverReply(ctx, id, sender)
	return err
}

func (a *App) sendFailureReply(ctx context.Context, sender mailbox.Sender, from, to, inReplyTo, name, kind, reason string) {
	if to == "" {
		return
	}
	if name == "" {
		name = "Command"
	}
	res := command.Result{
		Status:  "error",
		Summary: "MailRelay command rejected",
		Body:    fmt.Sprintf("Your command was not accepted: %s.", reason),
	}
	reply, err := mailbox.BuildReply(from, to, inReplyTo, name, res)
	if err != nil {
		_ = a.store.AddEvent(ctx, store.RuntimeEvent{Severity: "error", Phase: "reply", ErrorKind: kind, Summary: summaryWithDetail("failure reply could not be built", err)})
		slog.Warn("failure reply could not be built", "kind", kind, "error", safeErrorDetail(err), "error_type", fmt.Sprintf("%T", err))
		return
	}
	if err = sender.Send(ctx, to, reply); err != nil {
		_ = a.store.AddEvent(ctx, store.RuntimeEvent{Severity: "error", Phase: "reply", ErrorKind: "dependency", Summary: summaryWithDetail(runtimeEventSummary("reply"), err)})
		slog.Warn("failure reply delivery failed", "kind", kind, "error", safeErrorDetail(err), "error_type", fmt.Sprintf("%T", err))
		return
	}
	_ = a.store.AddEvent(ctx, store.RuntimeEvent{Severity: "info", Phase: "reply", ErrorKind: kind, Summary: "failure reply sent"})
}
func (a *App) deliverReply(ctx context.Context, id int64, sender mailbox.Sender) (bool, error) {
	r, err := a.store.ClaimReplyID(ctx, id, time.Now(), 30*time.Second)
	if err != nil || r == nil {
		return false, err
	}
	if err = sender.Send(ctx, r.Recipient, r.Payload); err != nil {
		_ = a.store.AddEvent(ctx, store.RuntimeEvent{Severity: "error", Phase: "reply", MessageID: r.MessageID, ErrorKind: "dependency", Summary: summaryWithDetail(runtimeEventSummary("reply"), err)})
		slog.Warn("reply delivery failed", "message_id", r.MessageID, "attempt", r.Attempts, "max_attempts", r.MaxAttempts, "error", safeErrorDetail(err), "error_type", fmt.Sprintf("%T", err))
		if e := a.store.FailReply(ctx, r, err.Error(), a.retryBackoff(r.Attempts)); e != nil {
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
		_ = a.store.AddEvent(ctx, store.RuntimeEvent{Severity: "error", Phase: "reply", MessageID: r.MessageID, ErrorKind: "dependency", Summary: summaryWithDetail(runtimeEventSummary("reply"), err)})
		slog.Warn("reply delivery failed", "message_id", r.MessageID, "attempt", r.Attempts, "max_attempts", r.MaxAttempts, "error", safeErrorDetail(err), "error_type", fmt.Sprintf("%T", err))
		if e := a.store.FailReply(ctx, r, err.Error(), a.retryBackoff(r.Attempts)); e != nil {
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

func (a *App) PreviewMail(raw string) MailPreview {
	m, err := mailparse.Parse(strings.NewReader(raw))
	if err != nil {
		return MailPreview{Stage: "parse", ErrorKind: "parse"}
	}

	a.mu.RLock()
	allow := append([]string(nil), a.allow...)
	token, from, route := a.token, a.from, a.router
	a.mu.RUnlock()
	if sameMailboxAddress(m.Request.Sender, from) {
		return MailPreview{Stage: "auth", Command: m.Request.Name, ErrorKind: "self_message"}
	}
	if err = security.Authenticate(m.Request, m.Token, allow, token); err != nil {
		kind := "authentication"
		if safeAuthReason(err) == "invalid token" {
			kind = "token"
		} else if safeAuthReason(err) == "sender not allowed" {
			kind = "sender"
		}
		return MailPreview{Stage: "auth", Command: m.Request.Name, ErrorKind: kind}
	}
	if m.Request.Name == "help" {
		return MailPreview{Accepted: true, Stage: "route", Command: "help", Handler: "builtin", Parameters: sortedParameterNames(m.Request.Params)}
	}
	c, ok := route.Command(m.Request.Name)
	if !ok {
		return MailPreview{Stage: "route", Command: m.Request.Name, ErrorKind: "unknown_command"}
	}
	params, err := command.ValidateParams(c, m.Request.Params)
	if err != nil {
		return MailPreview{Stage: "parameters", Command: c.Name, Handler: c.Handler, ErrorKind: "invalid_parameters"}
	}
	return MailPreview{Accepted: true, Stage: "ready", Command: c.Name, Handler: c.Handler, Parameters: sortedParameterNames(params)}
}

func sortedParameterNames(params map[string]any) []string {
	names := make([]string, 0, len(params))
	for name := range params {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
func (a *App) retryBackoff(attempt int) time.Duration {
	a.mu.RLock()
	initial, max := a.initialBackoff, a.maxBackoff
	a.mu.RUnlock()
	return retryBackoff(initial, max, attempt)
}
func (a *App) Execute(ctx context.Context, req command.Request) (command.Result, error) {
	a.mu.RLock()
	r := a.router
	a.mu.RUnlock()
	return r.Execute(ctx, req)
}
func retryBackoff(initial, max time.Duration, attempt int) time.Duration {
	if initial <= 0 {
		initial = time.Minute
	}
	if max <= 0 {
		max = initial
	}
	if attempt < 1 {
		attempt = 1
	}
	backoff := initial
	for i := 1; i < attempt; i++ {
		if backoff >= max/2 {
			backoff = max
			break
		}
		backoff *= 2
	}
	if backoff > max {
		return max
	}
	return backoff
}
func classify(err error) string {
	var e *command.Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return "internal"
}

func isMessageLevelError(err error) bool {
	var e *command.Error
	if !errors.As(err, &e) {
		return false
	}
	switch e.Kind {
	case "parse", "authentication", "unknown_command", "invalid_parameters", "policy":
		return true
	default:
		return false
	}
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
	slog.Debug("mail poll started", "limit", limit)
	msgs, err := r.Fetch(ctx, limit)
	if err != nil {
		return err
	}
	if len(msgs) == 0 {
		slog.Info("mail poll completed", "unseen", 0)
		return nil
	}
	slog.Info("mail poll fetched messages", "unseen", len(msgs))
	for _, m := range msgs {
		err = a.Process(ctx, mailbox.RawReader(m))
		if err != nil {
			if !isMessageLevelError(err) {
				return err
			}
			kind := classify(err)
			messageID := fmt.Sprintf("uid:%d", m.UID)
			commandName := ""
			if kind == "parse" {
				env, envErr := mailparse.ParseEnvelope(mailbox.RawReader(m))
				if envErr == nil {
					messageID = env.MessageID
					commandName = env.Name
					if _, execErr := a.store.AddExecution(ctx, store.Execution{
						MessageID: messageID,
						Command:   commandName,
						Handler:   "mail",
						Status:    "error",
						Summary:   "parse failed",
						Error:     "parse",
						StartedAt: time.Now(),
					}, nil); execErr != nil {
						return execErr
					}
					a.mu.RLock()
					from, sender := a.from, a.sender
					a.mu.RUnlock()
					if sameMailboxAddress(env.Sender, from) {
						slog.Warn("self-generated malformed mail message ignored", "uid", m.UID, "message_id", messageID)
					} else {
						a.sendFailureReply(ctx, sender, from, env.Sender, env.InReplyTo, env.Name, "parse", "parse failed")
					}
				} else {
					if _, execErr := a.store.AddExecution(ctx, store.Execution{
						MessageID: messageID,
						Command:   "mail",
						Handler:   "mail",
						Status:    "error",
						Summary:   "parse failed",
						Error:     "parse",
						StartedAt: time.Now(),
					}, nil); execErr != nil {
						return execErr
					}
				}
			}
			if update, ok := messageFailureForUID(m.UID, err); ok {
				if commandName != "" {
					update.Command = commandName
				}
				if recordErr := a.store.RecordMessageFailure(ctx, update); recordErr != nil {
					return recordErr
				}
			}
			_ = a.store.AddEvent(ctx, store.RuntimeEvent{
				Severity:  "warn",
				Phase:     "message",
				MessageID: messageID,
				Command:   commandName,
				ErrorKind: kind,
				Summary:   "message discarded",
			})
			slog.Warn("mail message discarded", "uid", m.UID, "reason", kind)
		}
		if markErr := r.MarkSeen(ctx, m.UID); markErr != nil {
			return markErr
		}
		slog.Info("mail message marked seen", "uid", m.UID)
	}
	return nil
}
func IgnoreNoRows(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return err
}
