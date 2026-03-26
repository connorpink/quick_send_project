package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestHostsJSON(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(configPath, []byte(`
[defaults]
extract = true
compression = "gzip"
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
