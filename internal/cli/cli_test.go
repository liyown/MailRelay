package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/becomeopc/opc-mailrelay/internal/config"
	"github.com/becomeopc/opc-mailrelay/internal/store"
)

func TestInitHelpDoctorStatus(t *testing.T) {
	d := t.TempDir()
	cfg := filepath.Join(d, "mailrelay.yaml")
	var out, errout bytes.Buffer
	if code := Run(context.Background(), []string{"--config", cfg, "init"}, &out, &errout); code != 0 {
		t.Fatalf("init=%d %s", code, errout.String())
	}
	b, err := os.ReadFile(cfg)
	if err != nil || !strings.Contains(string(b), "commands:") {
		t.Fatalf("%v %s", err, b)
	}
	out.Reset()
	errout.Reset()
	if code := Run(context.Background(), []string{"--config", cfg, "doctor"}, &out, &errout); code != 0 {
		t.Fatalf("doctor=%d %s", code, errout.String())
	}
	if !strings.Contains(out.String(), "configuration: ok") {
		t.Fatal(out.String())
	}
	out.Reset()
	if code := Run(context.Background(), []string{"--config", cfg, "status"}, &out, &errout); code != 0 {
		t.Fatalf("status=%d %s", code, errout.String())
	}
	if !strings.Contains(out.String(), "queue_depth:") {
		t.Fatal(out.String())
	}
	out.Reset()
	if code := Run(context.Background(), []string{"help"}, &out, &errout); code != 0 || !strings.Contains(out.String(), "mailrelay run") {
		t.Fatal(out.String())
	}
}

func TestStatusAndReplayDeadLetters(t *testing.T) {
	d := t.TempDir()
	cfgPath := filepath.Join(d, "mailrelay.yaml")
	var out, errout bytes.Buffer
	if code := Run(context.Background(), []string{"--config", cfgPath, "init"}, &out, &errout); code != 0 {
		t.Fatal(errout.String())
	}
	c, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(c.Storage.Path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	jobID, _ := s.Enqueue(ctx, "deploy", nil, "cli-dead", 1, time.Now())
	j, _ := s.ClaimJob(ctx, time.Now(), time.Minute)
	_ = s.FailJob(ctx, j, "job failed", 0)
	replyID, _ := s.RecordExecutionAndReply(ctx, store.Execution{MessageID: "m", Command: "x", Status: "success", StartedAt: time.Now()}, nil, "me@example.com", []byte("reply"), 1)
	rp, _ := s.ClaimReply(ctx, time.Now(), time.Minute)
	_ = s.FailReply(ctx, rp, "smtp failed", 0)
	_ = s.Close()
	out.Reset()
	errout.Reset()
	if code := Run(ctx, []string{"--config", cfgPath, "status"}, &out, &errout); code != 0 {
		t.Fatal(errout.String())
	}
	for _, want := range []string{"queue_pending:", "queue_running:", "reply_running:", "stale_executing:", "recent_failure:", "queue_dead: 1", "reply_dead: 1"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in %s", want, out.String())
		}
	}
	out.Reset()
	errout.Reset()
	if code := Run(ctx, []string{"--config", cfgPath, "replay", "queue", fmt.Sprint(jobID)}, &out, &errout); code != 0 {
		t.Fatal(errout.String())
	}
	if code := Run(ctx, []string{"--config", cfgPath, "replay", "reply", fmt.Sprint(replyID)}, &out, &errout); code != 0 {
		t.Fatal(errout.String())
	}
}
func TestUsageError(t *testing.T) {
	var out, errout bytes.Buffer
	if code := Run(context.Background(), []string{"unknown"}, &out, &errout); code != 2 {
		t.Fatalf("code=%d", code)
	}
}

func TestVersionAndZeroDurationSoak(t *testing.T) {
	var out, errout bytes.Buffer
	if code := Run(context.Background(), []string{"version"}, &out, &errout); code != 0 || !strings.Contains(out.String(), "mailrelay dev") {
		t.Fatalf("%s %s", out.String(), errout.String())
	}
	d := t.TempDir()
	cfg := filepath.Join(d, "mailrelay.yaml")
	out.Reset()
	if code := Run(context.Background(), []string{"--config", cfg, "init"}, &out, &errout); code != 0 {
		t.Fatal(errout.String())
	}
	out.Reset()
	errout.Reset()
	if code := Run(context.Background(), []string{"--config", cfg, "soak", "--duration", "0s"}, &out, &errout); code != 0 {
		t.Fatalf("%s", errout.String())
	}
	for _, want := range []string{"duration: 0s", "queue_dead:", "reply_dead:", "reply_pending:", "soak_result: pass"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in %s", want, out.String())
		}
	}
}

func TestDoctorLabelsLocalChecksAndSkipsNetworkByDefault(t *testing.T) {
	d := t.TempDir()
	cfg := filepath.Join(d, "mailrelay.yaml")
	var out, errout bytes.Buffer
	if code := Run(context.Background(), []string{"--config", cfg, "init"}, &out, &errout); code != 0 {
		t.Fatal(errout.String())
	}
	out.Reset()
	errout.Reset()
	if code := Run(context.Background(), []string{"--config", cfg, "doctor"}, &out, &errout); code != 0 {
		t.Fatal(errout.String())
	}
	for _, want := range []string{"local checks:", "network checks: skipped"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in %s", want, out.String())
		}
	}
}

func TestDoctorWarnsForExperimentalHandler(t *testing.T) {
	d := t.TempDir()
	cfg := filepath.Join(d, "mailrelay.yaml")
	var out, errout bytes.Buffer
	if code := Run(context.Background(), []string{"--config", cfg, "init"}, &out, &errout); code != 0 {
		t.Fatal(errout.String())
	}
	f, err := os.OpenFile(cfg, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	body, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(body), "  config_reload: true\n", "  config_reload: true\n  enable_experimental: true\n", 1)
	updated = strings.Replace(updated, "commands:\n", "commands:\n  - name: local\n    description: local\n    handler: shell\n    config: {executable: /bin/echo}\n", 1)
	if err := os.WriteFile(cfg, []byte(updated), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errout.Reset()
	if code := Run(context.Background(), []string{"--config", cfg, "doctor"}, &out, &errout); code != 0 {
		t.Fatalf("%s", errout.String())
	}
	if !strings.Contains(out.String(), "WARNING: shell is Experimental") || !strings.Contains(out.String(), "executable /bin/echo: ok") {
		t.Fatal(out.String())
	}
}
