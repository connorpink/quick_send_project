package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAndResolveHost(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[defaults]
extract = true
send_transfer_mode = "auto"
remote_temp_dir = "/tmp/sendrecv"
rsync_args = ["--archive"]
ssh_args = ["-o", "BatchMode=yes"]

[tools]
ssh = "ssh"
rsync = "rsync"

[hosts.dev]
ssh_target = "user@dev"
remote_dir = "/srv/drop"
sendrecv_path = "/usr/local/bin/sendrecv"
remote_rsync_path = "/usr/local/bin/rsync"
extract = false
rsync_args = ["--info=progress2"]
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	host, err := cfg.ResolveHost("dev")
	if err != nil {
		t.Fatal(err)
	}
	if host.RemoteDir != "/srv/drop" {
		t.Fatalf("remote dir mismatch: %s", host.RemoteDir)
	}
	if host.Extract {
		t.Fatalf("host override should disable extract")
	}
	if host.SendrecvPath != "/usr/local/bin/sendrecv" {
		t.Fatalf("sendrecv path mismatch: %s", host.SendrecvPath)
	}
	if host.RemoteRsyncPath != "/usr/local/bin/rsync" {
		t.Fatalf("remote rsync path mismatch: %s", host.RemoteRsyncPath)
	}
	if got := len(host.RsyncArgs); got != 2 {
		t.Fatalf("expected merged rsync args, got %d", got)
	}
}

func TestValidateRejectsRelativeRemoteDir(t *testing.T) {
	cfg := &Config{
		Defaults: Defaults{SendTransferMode: SendTransferModeAuto, RemoteTempDir: "/tmp/sendrecv"},
		Tools:    Tools{SSH: "ssh", RSync: "rsync"},
		Hosts: map[string]*Host{
			"bad": {SSHTarget: "u@h", RemoteDir: "relative"},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestLoadRejectsLegacyTarFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[defaults]
extract = true
send_transfer_mode = "auto"

[tools]
ssh = "ssh"
rsync = "rsync"
tar = "tar"

[hosts.dev]
ssh_target = "user@dev"
remote_dir = "/srv/drop"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected unknown legacy field to fail")
	}
}

func TestRenderQuotesHostKeysWhenNeeded(t *testing.T) {
	cfg := MinimalConfig()
	cfg.Hosts["linux.server"] = &Host{
		SSHTarget:       "alice@devbox.example",
		SendrecvPath:    "sendrecv",
		RemoteRsyncPath: "/usr/local/bin/rsync",
		RemoteDir:       "/srv/incoming",
		RemoteTempDir:   "/srv/incoming/tmp",
	}
	rendered := cfg.Render()
	if !strings.Contains(rendered, `[hosts."linux.server"]`) {
		t.Fatalf("expected quoted host key, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, `remote_rsync_path = "/usr/local/bin/rsync"`) {
		t.Fatalf("expected remote_rsync_path in rendered config, got:\n%s", rendered)
	}
}

func TestValidateAcceptsBareRemoteRsyncPath(t *testing.T) {
	cfg := &Config{
		Defaults: Defaults{SendTransferMode: SendTransferModeAuto, RemoteTempDir: "/tmp/sendrecv"},
		Tools:    Tools{SSH: "ssh", RSync: "rsync"},
		Hosts: map[string]*Host{
			"ok": {SSHTarget: "u@h", RemoteDir: "/srv/drop", RemoteRsyncPath: "rsync-custom"},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected validation success, got %v", err)
	}
}

func TestValidateRejectsInvalidRemoteRsyncPath(t *testing.T) {
	cfg := &Config{
		Defaults: Defaults{SendTransferMode: SendTransferModeAuto, RemoteTempDir: "/tmp/sendrecv"},
		Tools:    Tools{SSH: "ssh", RSync: "rsync"},
		Hosts: map[string]*Host{
			"bad": {SSHTarget: "u@h", RemoteDir: "/srv/drop", RemoteRsyncPath: "bin/rsync custom"},
		},
	}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "remote_rsync_path") {
		t.Fatalf("expected remote_rsync_path validation error, got %v", err)
	}
}

func TestLoadAcceptsLegacyCompressionField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[defaults]
extract = true
compression = "gzip"
remote_temp_dir = "/tmp/sendrecv"

[tools]
ssh = "ssh"
rsync = "rsync"

[hosts.dev]
ssh_target = "user@dev"
remote_dir = "/srv/drop"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Defaults.SendTransferMode != SendTransferModeAuto {
		t.Fatalf("expected legacy compression to map to auto mode, got %q", cfg.Defaults.SendTransferMode)
	}
}

func TestValidateRejectsUnknownSendTransferMode(t *testing.T) {
	cfg := &Config{
		Defaults: Defaults{SendTransferMode: SendTransferMode("fast"), RemoteTempDir: "/tmp/sendrecv"},
		Tools:    Tools{SSH: "ssh", RSync: "rsync"},
		Hosts: map[string]*Host{
			"ok": {SSHTarget: "u@h", RemoteDir: "/srv/drop"},
		},
	}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "send_transfer_mode") {
		t.Fatalf("expected send_transfer_mode validation error, got %v", err)
	}
}

func TestRenderWritesSendTransferMode(t *testing.T) {
	cfg := MinimalConfig()
	cfg.Hosts["dev"] = &Host{SSHTarget: "user@dev", RemoteDir: "/srv/drop"}

	rendered := cfg.Render()
	if !strings.Contains(rendered, `send_transfer_mode = "auto"`) {
		t.Fatalf("expected send_transfer_mode in rendered config, got:\n%s", rendered)
	}
	if strings.Contains(rendered, `compression = "gzip"`) {
		t.Fatalf("did not expect legacy compression field in rendered config, got:\n%s", rendered)
	}
}
