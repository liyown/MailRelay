package cli

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/becomeopc/opc-mailrelay/internal/app"
	"github.com/becomeopc/opc-mailrelay/internal/command"
	"github.com/becomeopc/opc-mailrelay/internal/config"
	"github.com/becomeopc/opc-mailrelay/internal/security"
	"github.com/becomeopc/opc-mailrelay/internal/store"
	"github.com/becomeopc/opc-mailrelay/internal/version"
)

const usage = `MailRelay - email-driven command runtime

Usage:
	mailrelay init [--config path]
	mailrelay run [--config path]
	mailrelay once [--config path]
	mailrelay status [--config path]
	mailrelay doctor [--config path]
	mailrelay replay queue|reply ID [--config path]
	mailrelay soak --duration 72h [--config path]
	mailrelay version
  mailrelay help
`

func Run(ctx context.Context, args []string, out, errout io.Writer) int {
	// Accept --config before or after the command while keeping the command surface small.
	for i := 1; i < len(args); i++ {
		if args[i] == "--config" && i+1 < len(args) {
			args = append([]string{"--config", args[i+1]}, append(args[:i], args[i+2:]...)...)
			break
		}
	}
	fs := flag.NewFlagSet("mailrelay", flag.ContinueOnError)
	fs.SetOutput(errout)
	path := fs.String("config", configPath(), "configuration path")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	rest := fs.Args()
	if len(rest) == 0 || rest[0] == "help" {
		fmt.Fprint(out, usage)
		return 0
	}
	var err error
	switch rest[0] {
	case "init":
		err = initConfig(*path, out)
	case "doctor":
		err = doctor(*path, out)
	case "status":
		err = status(*path, out)
	case "once":
		err = runOnce(ctx, *path)
	case "run":
		err = run(ctx, *path)
	case "replay":
		err = replay(*path, rest[1:], out)
	case "soak":
		err = soak(ctx, *path, rest[1:], out)
	case "version":
		fmt.Fprintln(out, version.String())
	default:
		fmt.Fprintf(errout, "unknown command %q\n%s", rest[0], usage)
		return 2
	}
	if err != nil {
		fmt.Fprintln(errout, err)
		return 1
	}
	return 0
}
func configPath() string {
	if p := os.Getenv("MAILRELAY_CONFIG"); p != "" {
		return p
	}
	return "mailrelay.yaml"
}
func initConfig(path string, out io.Writer) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("configuration already exists: %s", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(filepath.Dir(path), "data"), 0700); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(example), 0600); err != nil {
		return err
	}
	fmt.Fprintf(out, "created %s\n", path)
	return nil
}
func doctor(path string, out io.Writer) error {
	c, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}
	fmt.Fprintln(out, "configuration: ok")
	for name, address := range map[string]string{"imap": c.Mail.IMAP.Address, "smtp": c.Mail.SMTP.Address} {
		if address == "" {
			return fmt.Errorf("%s address is required", name)
		}
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return fmt.Errorf("%s address: %w", name, err)
		}
		if host == "" {
			return fmt.Errorf("%s host is empty", name)
		}
		fmt.Fprintf(out, "%s: configured\n", name)
	}
	s, err := store.Open(c.Storage.Path)
	if err != nil {
		return fmt.Errorf("sqlite: %w", err)
	}
	defer s.Close()
	fmt.Fprintln(out, "sqlite: ok")
	seenMaturity := map[string]bool{}
	for _, cmd := range c.Commands {
		m := command.HandlerMaturity(cmd.Handler)
		if m != "Stable" && !seenMaturity[cmd.Handler] {
			fmt.Fprintf(out, "WARNING: %s is %s\n", cmd.Handler, m)
			seenMaturity[cmd.Handler] = true
		}
		if cmd.Handler == "shell" || cmd.Handler == "plugin" {
			exe, _ := cmd.Config["executable"].(string)
			st, e := os.Stat(exe)
			if e != nil {
				return fmt.Errorf("executable %s: %w", exe, e)
			}
			if !st.Mode().IsRegular() {
				return fmt.Errorf("executable %s is not a regular file", exe)
			}
			fmt.Fprintf(out, "executable %s: ok\n", exe)
		}
	}
	policy := security.NetworkPolicy{Hosts: c.Security.HTTPHosts}
	for _, host := range c.Security.HTTPHosts {
		u := &url.URL{Scheme: "https", Host: host}
		if e := policy.Check(context.Background(), u); e != nil {
			fmt.Fprintf(out, "WARNING: outbound host %s: %v\n", host, e)
		} else {
			fmt.Fprintf(out, "outbound host %s: ok\n", host)
		}
	}
	fmt.Fprintf(out, "commands: %d\n", len(c.Commands))
	return nil
}
func status(path string, out io.Writer) error {
	c, err := config.Load(path)
	if err != nil {
		return err
	}
	s, err := store.Open(c.Storage.Path)
	if err != nil {
		return err
	}
	defer s.Close()
	ctx := context.Background()
	depth, err := s.QueueDepth(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "queue_depth: %d\n", depth)
	h, err := s.Health(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "queue_pending: %d\nqueue_running: %d\nqueue_dead: %d\n", h.QueuePending, h.QueueRunning, h.QueueDead)
	fmt.Fprintf(out, "reply_pending: %d\nreply_running: %d\nreply_dead: %d\n", h.ReplyPending, h.ReplyRunning, h.ReplyDead)
	fmt.Fprintf(out, "stale_executing: %d\n", h.StaleExecuting)
	if len(h.LatestFailures) == 0 {
		fmt.Fprintln(out, "recent_failure: none")
	} else {
		for _, e := range h.LatestFailures {
			fmt.Fprintf(out, "recent_failure: %s %s %s\n", e.Phase, e.ErrorKind, e.Summary)
		}
	}
	lastPoll, stateErr := s.State(ctx, "last_poll")
	if errors.Is(stateErr, sql.ErrNoRows) || lastPoll == "" {
		lastPoll = "never"
	} else if stateErr != nil {
		return stateErr
	}
	runtimeErr, stateErr := s.State(ctx, "last_runtime_error")
	if errors.Is(stateErr, sql.ErrNoRows) || runtimeErr == "" {
		runtimeErr = "none"
	} else if stateErr != nil {
		return stateErr
	}
	fmt.Fprintf(out, "last_poll: %s\nruntime_error: %s\n", lastPoll, runtimeErr)
	if failure, e := s.LatestFailure(ctx); e == nil {
		fmt.Fprintf(out, "last_error: %s\n", failure)
	} else if errors.Is(e, sql.ErrNoRows) {
		fmt.Fprintln(out, "last_error: none")
	} else {
		return e
	}
	hash, _, notified, err := s.Catalog(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		hash = "not initialized"
		err = nil
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "catalog_hash: %s\ncatalog_notified: %v\n", hash, notified)
	e, err := s.RecentExecution(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		fmt.Fprintln(out, "last_execution: none")
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "last_execution: %s %s %s\n", e.Command, e.Status, e.StartedAt.Format(time.RFC3339))
	return nil
}
func replay(path string, args []string, out io.Writer) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: mailrelay replay queue|reply ID")
	}
	id, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil || id < 1 {
		return fmt.Errorf("invalid ID %q", args[1])
	}
	c, err := config.Load(path)
	if err != nil {
		return err
	}
	s, err := store.Open(c.Storage.Path)
	if err != nil {
		return err
	}
	defer s.Close()
	switch args[0] {
	case "queue":
		err = s.ReplayJob(context.Background(), id)
	case "reply":
		err = s.ReplayReply(context.Background(), id)
	default:
		return fmt.Errorf("replay type must be queue or reply")
	}
	if err == nil {
		fmt.Fprintf(out, "replayed %s %d\n", args[0], id)
	}
	return err
}
func soak(parent context.Context, path string, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("soak", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	duration := fs.Duration("duration", 72*time.Hour, "soak duration")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *duration < 0 {
		return fmt.Errorf("duration must not be negative")
	}
	r, err := app.Build(path)
	if err != nil {
		return err
	}
	defer r.Close()
	ctx, cancel := context.WithTimeout(parent, *duration)
	defer cancel()
	started := time.Now()
	if err = r.Run(ctx); err != nil {
		return err
	}
	queueDead, replyDead, err := r.Store().DeadCounts(context.Background())
	if err != nil {
		return err
	}
	pendingReplies, _, err := r.Store().ReplyCounts(context.Background())
	if err != nil {
		return err
	}
	result := "pass"
	if queueDead > 0 || replyDead > 0 || pendingReplies > 0 {
		result = "fail"
	}
	fmt.Fprintf(out, "duration: %s\nobserved: %s\nqueue_dead: %d\nreply_dead: %d\nreply_pending: %d\nsoak_result: %s\n", duration.String(), time.Since(started).Round(time.Millisecond), queueDead, replyDead, pendingReplies, result)
	if result != "pass" {
		return fmt.Errorf("soak invariants failed")
	}
	return nil
}
func runOnce(ctx context.Context, path string) error {
	r, err := app.Build(path)
	if err != nil {
		return err
	}
	defer r.Close()
	return r.Once(ctx)
}
func run(ctx context.Context, path string) error {
	r, err := app.Build(path)
	if err != nil {
		return err
	}
	defer r.Close()
	return r.Run(ctx)
}

const example = `mail:
  imap:
    address: imap.example.com:993
    username: relay@example.com
    password: change-me
    mailbox: INBOX
    poll_interval: 30s
  smtp:
    address: smtp.example.com:465
    username: relay@example.com
    password: change-me
    from: relay@example.com

security:
  token: change-me
  allow: [me@example.com]
  http_hosts: [api.example.com]

storage:
  path: data/mailrelay.db

runtime:
  command_timeout: 30s
  config_reload: true
  catalog_notify: [me@example.com]

handlers: {}

commands:
  - name: push
    description: Send a notification
    handler: http
    parameters:
      message: {description: Notification text, type: string, required: true, example: hello}
    config:
      method: POST
      url: https://api.example.com/push
      body: '{"message":"{{message}}"}'
`
