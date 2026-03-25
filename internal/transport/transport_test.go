package transport

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sendrecv/internal/config"
	"sendrecv/internal/doctor"
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
	if len(plan.Operations) != 2 {
		t.Fatalf("expected mkdir and raw rsync operations, got %d", len(plan.Operations))
	}
	if got := plan.Operations[1].Display(); !strings.Contains(got, "rsync") {
		t.Fatalf("unexpected operation: %s", got)
	}
}

func TestSendPlanDryRunIncludesFallbackNote(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "README.md")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig()
	host, _ := cfg.ResolveHost("box")
	runner := Runner{
		Config: cfg,
		Exec:   ExecOptions{DryRun: true},
	}
	plan, err := runner.SendPlan(host, []string{file}, TransferOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, operation := range plan.Operations {
		if strings.Contains(operation.Display(), "runtime note: sendrecv will try remote") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected dry-run fallback note in operations")
	}
}

func TestSendPlanUsesTarGzipFallbackWhenRemoteSendrecvMissing(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "README.md")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig()
	host, _ := cfg.ResolveHost("box")
	runner := Runner{
		Config: cfg,
		RemoteProbe: func(context.Context, *config.Config, *config.ResolvedHost) (doctor.RemoteCapabilities, error) {
			return doctor.RemoteCapabilities{
				RsyncOK:         true,
				SendrecvOK:      false,
				TarOK:           true,
				GzipOK:          true,
				RemoteDirOK:     true,
				RemoteTempDirOK: true,
			}, nil
		},
	}
	plan, err := runner.SendPlan(host, []string{file}, TransferOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var foundWarning, foundExtract bool
	for _, operation := range plan.Operations {
		display := operation.Display()
		if strings.Contains(display, "using remote tar+gzip extraction fallback") {
			foundWarning = true
		}
		if strings.Contains(display, "gzip -dc") && strings.Contains(display, "tar -xf") {
			foundExtract = true
		}
	}
	if !foundWarning || !foundExtract {
		t.Fatalf("expected tar+gzip fallback operations, got %#v", plan.Operations)
	}
}

func TestSendPlanUploadsArchiveToRemoteDirWhenNoExtractorExists(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "README.md")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig()
	host, _ := cfg.ResolveHost("box")
	runner := Runner{
		Config: cfg,
		RemoteProbe: func(context.Context, *config.Config, *config.ResolvedHost) (doctor.RemoteCapabilities, error) {
			return doctor.RemoteCapabilities{
				RsyncOK:         true,
				SendrecvOK:      false,
				TarOK:           false,
				GzipOK:          false,
				RemoteDirOK:     true,
				RemoteTempDirOK: true,
			}, nil
		},
	}
	plan, err := runner.SendPlan(host, []string{file}, TransferOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var foundResult bool
	for _, operation := range plan.Operations {
		display := operation.Display()
		if strings.Contains(display, "result: archive uploaded to /srv/drop/sendrecv-transfer.tar.gz") {
			foundResult = true
		}
		if strings.Contains(display, "rsync") && strings.Contains(display, "/tmp/sendrecv/sendrecv-transfer.tar.gz") {
			t.Fatalf("archive should not upload to remote temp dir when no extractor exists: %s", display)
		}
	}
	if !foundResult {
		t.Fatalf("expected final archive location in remote_dir")
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
	if got := len(plan.Operations); got != 5 {
		t.Fatalf("expected 5 commands, got %d", got)
	}
	if got := plan.Operations[0].Display(); !strings.Contains(got, "pack --output") {
		t.Fatalf("missing remote pack command: %s", got)
	}
	if got := plan.Operations[3].Display(); !strings.Contains(got, "unpack --archive") {
		t.Fatalf("missing local unpack operation: %s", got)
	}
}

func TestRecvPlanForRawFileUsesRsyncOnly(t *testing.T) {
	cfg := testConfig()
	host, _ := cfg.ResolveHost("box")
	runner := Runner{Config: cfg}
	plan, err := runner.RecvPlan(host, []string{"movie.mp4"}, TransferOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(plan.Operations); got != 2 {
		t.Fatalf("expected raw recv operations, got %d", got)
	}
	if got := plan.Operations[1].Display(); !strings.Contains(got, "rsync") {
		t.Fatalf("unexpected operation: %s", got)
	}
}

func testConfig() *config.Config {
	return &config.Config{
		Defaults: config.Defaults{
			Extract:       true,
			Compression:   "gzip",
			RemoteTempDir: "/tmp/sendrecv",
			RsyncArgs:     []string{"--archive"},
			SSHArgs:       []string{"-o", "BatchMode=yes"},
		},
		Tools: config.Tools{SSH: "ssh", RSync: "rsync"},
		Hosts: map[string]*config.Host{
			"box": {SSHTarget: "me@box", RemoteDir: "/srv/drop", SendrecvPath: "sendrecv"},
		},
	}
}
