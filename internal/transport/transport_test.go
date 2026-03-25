package transport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sendrecv/internal/config"
)

func TestSendPlanForRawFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "video.mp4")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig()
	host, _ := cfg.ResolveHost("box")
	runner := Runner{Config: cfg}
	plan, err := runner.SendPlan(host, []string{file}, TransferOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Commands) != 1 {
		t.Fatalf("expected raw rsync command, got %d", len(plan.Commands))
	}
	if plan.Commands[0].Name != "rsync" {
		t.Fatalf("unexpected command: %+v", plan.Commands[0])
	}
}

func TestRecvPlanIncludesExtractAndCleanup(t *testing.T) {
	cfg := testConfig()
	host, _ := cfg.ResolveHost("box")
	runner := Runner{Config: cfg}
	plan, err := runner.RecvPlan(host, []string{"nested/file.txt"}, TransferOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(plan.Commands); got != 5 {
		t.Fatalf("expected 5 commands, got %d", got)
	}
	if !strings.Contains(plan.Commands[0].Args[len(plan.Commands[0].Args)-1], "tar -C") {
		t.Fatalf("missing remote archive command: %+v", plan.Commands[0])
	}
}

func testConfig() *config.Config {
	return &config.Config{
		Defaults: config.Defaults{
			Extract:       true,
			Compression:   "xz",
			RemoteTempDir: "/tmp/sendrecv",
			RsyncArgs:     []string{"--archive"},
			SSHArgs:       []string{"-o", "BatchMode=yes"},
		},
		Tools: config.Tools{SSH: "ssh", RSync: "rsync", Tar: "tar", XZ: "xz"},
		Hosts: map[string]*config.Host{
			"box": {SSHTarget: "me@box", RemoteDir: "/srv/drop"},
		},
	}
}
