package cli

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/becomeopc/opc-mailrelay/internal/app"
	"github.com/becomeopc/opc-mailrelay/internal/config"
	"github.com/becomeopc/opc-mailrelay/internal/store"
)

const usage = `MailRelay - email-driven command runtime

Usage:
	mailrelay init [--config path]
	mailrelay run [--config path]
	mailrelay once [--config path]
	mailrelay status [--config path]
	mailrelay doctor [--config path]
	mailrelay replay queue|reply ID [--config path]
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
	pendingReplies, deadReplies, err := s.ReplyCounts(ctx)
	if err != nil {
		return err
	}
	queueDead, _, err := s.DeadCounts(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "reply_pending: %d\nqueue_dead: %d\nreply_dead: %d\n", pendingReplies, queueDead, deadReplies)
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
