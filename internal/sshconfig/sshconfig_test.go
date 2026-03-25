package sshconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadImportsSimpleHosts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	data := `
Host linuxserver
  HostName linuxserver
  User cpink

Host wildcard-*
  HostName ignored
  User ignored

Match host something
  User ignored

Host missing-user
  HostName example
`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	hosts, skipped, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(hosts))
	}
	if hosts[0].Alias != "linuxserver" || hosts[0].User != "cpink" || hosts[0].HostName != "linuxserver" {
		t.Fatalf("unexpected host: %+v", hosts[0])
	}
	if len(skipped) == 0 {
		t.Fatal("expected skipped entries")
	}
}
