package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
func TestUsageError(t *testing.T) {
	var out, errout bytes.Buffer
	if code := Run(context.Background(), []string{"unknown"}, &out, &errout); code != 2 {
		t.Fatalf("code=%d", code)
	}
}
