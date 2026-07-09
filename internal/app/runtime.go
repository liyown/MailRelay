package app

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	"github.com/becomeopc/opc-mailrelay/internal/version"
	webconsole "github.com/becomeopc/opc-mailrelay/internal/web"
)

type Runtime struct {
	mu         sync.RWMutex
	reloadMu   sync.Mutex
	path       string
	cfg        *config.Config
	store      *store.Store
	app        *App
	receiver   mailbox.Receiver
	sender     mailbox.Sender
	registry   *handler.Registry
	custom     []command.Handler
	mtime      time.Time
	webAddress string
	repo       *webconsole.Repository
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
	r.app.SetRetryPolicy(cfg.Runtime.ReplyMaxAttempts, cfg.Runtime.InitialBackoff, cfg.Runtime.MaxBackoff)
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
	queue := handler.NewQueue(s)
	queue.SetRetryPolicy(cfg.Runtime.QueueMaxAttempts, cfg.Runtime.InitialBackoff, cfg.Runtime.MaxBackoff)
	builtins := []command.Handler{handler.NewHTTP(client, policy), handler.NewWebhook(client, policy), handler.NewWorkflow(64, 8), handler.NewPlugin(), handler.NewShell(), handler.NewAgent(client), handler.NewMCP(client), queue}
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
func (r *Runtime) WebAddress() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.webAddress
}
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
		r.mu.RLock()
		initialBackoff, maxBackoff := r.cfg.Runtime.InitialBackoff, r.cfg.Runtime.MaxBackoff
		r.mu.RUnlock()
		worked, err := handler.RunOneJobWithPolicy(ctx, r.store, a, 30*time.Second, initialBackoff, maxBackoff)
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
	r.mu.RLock()
	webConfig := r.cfg.Web
	commands := append([]command.Command(nil), r.cfg.Commands...)
	r.mu.RUnlock()
	r.logStartup()
	if !webConfig.Enabled {
		return r.runMail(ctx)
	}
	sessions, err := webconsole.NewSessionManager(webconsole.SessionOptions{
		Secret:       []byte(webConfig.SessionSecret),
		PasswordHash: webConfig.AdminPasswordHash,
		TTL:          webConfig.SessionTTL,
		SecureCookie: strings.HasPrefix(webConfig.PublicURL, "https://"),
	})
	if err != nil {
		return fmt.Errorf("web authentication: %w", err)
	}
	listener, err := net.Listen("tcp", webConfig.Address)
	if err != nil {
		return fmt.Errorf("web listen: %w", err)
	}
	r.mu.Lock()
	r.webAddress = listener.Addr().String()
	r.mu.Unlock()
	slog.Info("web console listening", "url", consoleURL(webConfig, listener.Addr().String()))
	defer func() {
		r.mu.Lock()
		r.webAddress = ""
		r.mu.Unlock()
	}()

	server := &http.Server{
		Handler:           webconsole.NewServer(webconsole.ServerOptions{Sessions: sessions, Repository: r.newRepository(commands), Editor: r}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	mailErr := make(chan error, 1)
	webErr := make(chan error, 1)
	go func() { mailErr <- r.runMail(runCtx) }()
	go func() {
		err := server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		webErr <- err
	}()

	var result error
	select {
	case <-ctx.Done():
	case result = <-mailErr:
	case result = <-webErr:
	}
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); result == nil && err != nil {
		result = err
	}
	return result
}

func (r *Runtime) runMail(ctx context.Context) error {
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return nil
		}
		if err := r.reloadIfChanged(ctx); err != nil {
			slog.Error("configuration reload rejected", "error", err)
		}
		if err := r.Once(ctx); err != nil {
			r.mu.RLock()
			imapAddr := r.cfg.Mail.IMAP.Address
			r.mu.RUnlock()
			cause, hint := mailConnectionError(err)
			_ = r.store.AddEvent(context.Background(), store.RuntimeEvent{Severity: "error", Phase: "receiver", ErrorKind: "dependency", Summary: runtimeEventSummary("receiver")})
			_ = r.store.SetState(context.Background(), "last_runtime_error", runtimeEventSummary("receiver"))
			slog.Error("mail poll failed",
				"cause", cause,
				"imap", imapAddr,
				"error_type", fmt.Sprintf("%T", err),
				"retry_in", backoff.String(),
				"hint", hint,
			)
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
			cause, _ := mailConnectionError(err)
			slog.Warn("IMAP IDLE unavailable; polling fallback active", "cause", cause)
		}
	}
}

// logStartup emits a one-line operator banner so it is obvious which endpoints,
// storage, and command set the process came up with. Only host:port addresses
// are logged — never usernames, passwords, or tokens.
func (r *Runtime) logStartup() {
	r.mu.RLock()
	cfg := r.cfg
	r.mu.RUnlock()
	slog.Info("mailrelay started",
		"version", version.String(),
		"config", r.path,
		"imap", cfg.Mail.IMAP.Address,
		"smtp", cfg.Mail.SMTP.Address,
		"poll_interval", cfg.Mail.IMAP.PollInterval.String(),
		"storage", cfg.Storage.Path,
		"commands", len(cfg.Commands),
		"web", cfg.Web.Enabled,
	)
}

// consoleURL is the address an operator should open to reach the web console.
// It prefers the configured public URL and otherwise builds one from the bound
// listen address (which may differ from the config when a :0 port is used).
func consoleURL(w config.Web, bound string) string {
	if w.PublicURL != "" {
		return w.PublicURL
	}
	if bound == "" {
		bound = w.Address
	}
	return "http://" + bound
}

// mailConnectionError classifies a receiver failure into a stable cause label
// and an actionable operator hint. It inspects error types and only matches
// against fixed substrings — it never returns any part of the raw error, which
// may carry credentials or recipient addresses that must stay out of logs.
func mailConnectionError(err error) (cause, hint string) {
	if err == nil {
		return "none", ""
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return "dns", "the mail 'address' host could not be resolved — it must be your provider's server host:port (e.g. imap.qq.com:993 / smtp.qq.com:465), not your email account"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout", "connecting to the mail server timed out — check the address, port, and that outbound access is allowed"
	}
	var recordErr tls.RecordHeaderError
	if errors.As(err, &recordErr) {
		return "tls", "the TLS handshake failed — this port may expect STARTTLS or plaintext; use an implicit-TLS port (993 for IMAP, 465 for SMTP)"
	}
	var authorityErr x509.UnknownAuthorityError
	var hostnameErr x509.HostnameError
	var certErr x509.CertificateInvalidError
	if errors.As(err, &authorityErr) || errors.As(err, &hostnameErr) || errors.As(err, &certErr) {
		return "certificate", "the mail server TLS certificate could not be verified — confirm the 'address' host matches the server certificate"
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return "connect", "could not reach the mail server — verify the address and port are correct and reachable"
	}
	switch msg := strings.ToLower(err.Error()); {
	case strings.Contains(msg, "no such host"), strings.Contains(msg, "server misbehaving"):
		return "dns", "the mail 'address' host could not be resolved — it must be your provider's server host:port, not your email account"
	case strings.Contains(msg, "timeout"), strings.Contains(msg, "deadline exceeded"):
		return "timeout", "connecting to the mail server timed out — check the address, port, and that outbound access is allowed"
	case strings.Contains(msg, "refused"), strings.Contains(msg, "unreachable"), strings.Contains(msg, "reset"):
		return "connect", "the mail server could not be reached — check the address and port are correct and reachable"
	case strings.Contains(msg, "certificate"), strings.Contains(msg, "x509"):
		return "certificate", "the mail server TLS certificate could not be verified — confirm the 'address' host matches the server certificate"
	case strings.Contains(msg, "handshake"), msg == "eof", strings.Contains(msg, "tls"):
		return "tls", "the server closed the TLS connection — confirm 'address' is your provider's mail server host:port over implicit TLS (993 for IMAP, 465 for SMTP), not a plain email address or a STARTTLS-only port"
	case strings.Contains(msg, "login"), strings.Contains(msg, "authenticate"), strings.Contains(msg, "credential"), strings.Contains(msg, "password"):
		return "auth", "the mail server rejected the login — verify the username and password (many providers require an app-specific password / authorization code and IMAP/SMTP to be enabled)"
	default:
		return "unknown", "verify the IMAP/SMTP address, credentials, and network; run 'mailrelay doctor' to check configuration"
	}
}

func (r *Runtime) reloadIfChanged(ctx context.Context) error {
	r.mu.RLock()
	reload := r.cfg.Runtime.ConfigReload
	r.mu.RUnlock()
	if !reload {
		return nil
	}
	r.reloadMu.Lock()
	defer r.reloadMu.Unlock()
	st, err := os.Stat(r.path)
	if err != nil {
		return err
	}
	r.mu.RLock()
	current := r.mtime
	r.mu.RUnlock()
	if !st.ModTime().After(current) {
		return nil
	}
	cfg, err := config.Load(r.path)
	if err != nil {
		_ = r.store.AddEvent(ctx, store.RuntimeEvent{Severity: "error", Phase: "reload", ErrorKind: "config", Summary: runtimeEventSummary("reload")})
		return err
	}
	return r.applyConfig(ctx, cfg, st.ModTime())
}

// applyConfig rebuilds the router from an already-validated config and swaps it,
// the mail clients, and the security settings in atomically, then refreshes the
// console's command snapshot. Shared by mtime-driven reloads and console edits;
// callers hold reloadMu so the two paths never interleave.
func (r *Runtime) applyConfig(ctx context.Context, cfg *config.Config, mtime time.Time) error {
	reg, route, err := buildRouter(cfg, r.store, r.custom)
	if err != nil {
		_ = r.store.AddEvent(ctx, store.RuntimeEvent{Severity: "error", Phase: "reload", ErrorKind: "config", Summary: runtimeEventSummary("reload")})
		return err
	}
	r.mu.RLock()
	old := r.cfg
	r.mu.RUnlock()
	if err = r.updateCatalog(ctx, old.Commands, cfg.Commands); err != nil {
		_ = r.store.AddEvent(ctx, store.RuntimeEvent{Severity: "error", Phase: "reload", ErrorKind: "config", Summary: runtimeEventSummary("reload")})
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
	r.app.SetRetryPolicy(cfg.Runtime.ReplyMaxAttempts, cfg.Runtime.InitialBackoff, cfg.Runtime.MaxBackoff)
	r.mtime = mtime
	repo := r.repo
	r.mu.Unlock()
	if repo != nil {
		repo.SetCommands(cfg.Commands)
	}
	return nil
}

// newRepository builds the console repository and remembers it so applyConfig
// can refresh its command list on every reload/edit.
func (r *Runtime) newRepository(commands []command.Command) *webconsole.Repository {
	repo := webconsole.NewRepository(r.store, commands, time.Now())
	r.mu.Lock()
	r.repo = repo
	r.mu.Unlock()
	return repo
}

// LoadDraft returns the console-editable slice of configuration with ${VAR}
// tokens intact. Implements webconsole.Editor.
func (r *Runtime) LoadDraft() (config.Draft, error) {
	return config.LoadDraft(r.path)
}

// ApplyDraft validates a console edit against the FULL config, persists it with
// a surgical node-level rewrite (preserving mail credentials, tokens, the
// session secret, ${VAR} refs, and comments), then hot-applies it. Invalid
// drafts return a *webconsole.DraftError and never touch the on-disk file.
// Implements webconsole.Editor.
func (r *Runtime) ApplyDraft(ctx context.Context, draft config.Draft) error {
	r.reloadMu.Lock()
	defer r.reloadMu.Unlock()

	original, err := os.ReadFile(r.path)
	if err != nil {
		return err
	}
	rendered, err := config.RenderDraft(original, draft)
	if err != nil {
		return &webconsole.DraftError{Message: "unable to render configuration: " + err.Error()}
	}

	// Validate against a temp copy in the same directory so relative paths and
	// ${VAR} resolution match the real file. Only rename over the live config
	// once config.Load (parse + full Validate) accepts it.
	tmp, err := os.CreateTemp(filepath.Dir(r.path), ".mailrelay-draft-*.yaml")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err = tmp.Write(rendered); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	cfg, err := config.Load(tmpPath)
	if err != nil {
		return &webconsole.DraftError{Message: "invalid configuration: " + err.Error()}
	}
	if err = os.Rename(tmpPath, r.path); err != nil {
		return err
	}
	st, err := os.Stat(r.path)
	if err != nil {
		return err
	}
	return r.applyConfig(ctx, cfg, st.ModTime())
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
