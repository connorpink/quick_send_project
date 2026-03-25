package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAndResolveHost(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[defaults]
extract = true
compression = "gzip"
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
	if got := len(host.RsyncArgs); got != 2 {
		t.Fatalf("expected merged rsync args, got %d", got)
	}
}

func TestValidateRejectsRelativeRemoteDir(t *testing.T) {
	cfg := &Config{
		Defaults: Defaults{Compression: "gzip", RemoteTempDir: "/tmp/sendrecv"},
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
compression = "gzip"

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
