package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHostsJSON(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(configPath, []byte(`
[defaults]
extract = true
send_transfer_mode = "auto"
remote_temp_dir = "/tmp/sendrecv"

[tools]
ssh = "ssh"
rsync = "rsync"

[hosts.alpha]
ssh_target = "me@alpha"
remote_dir = "/srv/alpha"

[hosts.beta]
ssh_target = "me@beta"
remote_dir = "/srv/beta"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"--config", configPath, "hosts", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	var payload []hostSummary
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(payload))
	}
	if payload[0].Name != "alpha" || payload[1].Name != "beta" {
		t.Fatalf("unexpected host ordering: %+v", payload)
	}
}

func TestSendNoCompressUsesRawTransfer(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	filePath := filepath.Join(dir, "folder")
	if err := os.MkdirAll(filePath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filePath, "file.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(`
[defaults]
extract = true
send_transfer_mode = "auto"
remote_temp_dir = "/tmp/sendrecv"
rsync_args = ["--archive"]

[tools]
ssh = "ssh"
rsync = "rsync"

[hosts.alpha]
ssh_target = "me@alpha"
remote_dir = "/srv/alpha"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"--config", configPath, "--dry-run", "send", "--remote-host", "alpha", "--no-compress", filePath})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	output := stdout.String()
	if strings.Contains(output, "sendrecv pack") {
		t.Fatalf("expected raw send dry-run output, got:\n%s", output)
	}
	if !strings.Contains(output, "rsync") {
		t.Fatalf("expected rsync command in output, got:\n%s", output)
	}
}

func TestSendRejectsConflictingTransferModeFlags(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	filePath := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(`
[defaults]
extract = true
send_transfer_mode = "auto"
remote_temp_dir = "/tmp/sendrecv"

[tools]
ssh = "ssh"
rsync = "rsync"

[hosts.alpha]
ssh_target = "me@alpha"
remote_dir = "/srv/alpha"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--config", configPath, "send", "--remote-host", "alpha", "--transfer-mode", "archive", "--no-compress", filePath})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("expected conflict error, got %v", err)
	}
}
