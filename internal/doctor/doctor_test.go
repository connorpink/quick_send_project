package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sendrecv/internal/config"
)

func TestLocalChecksMissingConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.toml")
	checks := LocalChecks(path)
	if len(checks) == 0 {
		t.Fatal("expected checks")
	}
	if checks[0].Name != "config_path" || checks[0].Status != "warning" {
		t.Fatalf("unexpected first check: %+v", checks[0])
	}
}

func TestConfigCheckExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	check := ConfigCheck(path)
	if check.Status != "ok" {
		t.Fatalf("expected ok status, got %+v", check)
	}
}

func TestYaziKeymapPathsIncludesXDGFallback(t *testing.T) {
	paths, err := yaziKeymapPaths()
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, path := range paths {
		if strings.HasSuffix(path, filepath.Join(".config", "yazi", "keymap.toml")) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected XDG yazi keymap path in %v", paths)
	}
}

func TestYaziPluginPathsIncludesPluginMainLua(t *testing.T) {
	paths, err := yaziPluginPaths("sendrecv.yazi")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, path := range paths {
		if strings.HasSuffix(path, filepath.Join(".config", "yazi", "plugins", "sendrecv.yazi", "main.lua")) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected plugin main.lua path in %v", paths)
	}
}

func TestParseRemoteCapabilitiesReadsResolvedRsyncPath(t *testing.T) {
	capabilities := parseRemoteCapabilities("rsync_path=/usr/local/bin/rsync\nrsync_source=fallback\nsendrecv_path=missing\n")
	if !capabilities.RsyncOK {
		t.Fatal("expected rsync to be available")
	}
	if capabilities.RsyncPath != "/usr/local/bin/rsync" {
		t.Fatalf("unexpected rsync path: %q", capabilities.RsyncPath)
	}
	if capabilities.RsyncSource != "fallback" {
		t.Fatalf("unexpected rsync source: %q", capabilities.RsyncSource)
	}
}

func TestRemoteRsyncCheckConfiguredPathFound(t *testing.T) {
	host := &config.ResolvedHost{RemoteRsyncPath: "/opt/tools/rsync"}
	check := remoteRsyncCheck(host, RemoteCapabilities{
		RsyncOK:     true,
		RsyncPath:   "/opt/tools/rsync",
		RsyncSource: "configured",
	})
	if check.Status != "ok" || !strings.Contains(check.Detail, "configured remote_rsync_path") {
		t.Fatalf("unexpected check: %+v", check)
	}
}

func TestRemoteRsyncCheckConfiguredPathMissing(t *testing.T) {
	host := &config.ResolvedHost{RemoteRsyncPath: "/opt/tools/rsync"}
	check := remoteRsyncCheck(host, RemoteCapabilities{})
	if check.Status != "warning" || !strings.Contains(check.Detail, host.RemoteRsyncPath) {
		t.Fatalf("unexpected check: %+v", check)
	}
}

func TestRemoteRsyncCheckFallbackPathFound(t *testing.T) {
	check := remoteRsyncCheck(&config.ResolvedHost{}, RemoteCapabilities{
		RsyncOK:     true,
		RsyncPath:   "/usr/local/bin/rsync",
		RsyncSource: "fallback",
	})
	if check.Status != "ok" || !strings.Contains(check.Detail, "/usr/local/bin/rsync") {
		t.Fatalf("unexpected check: %+v", check)
	}
}

func TestRemoteChecksFromCapabilitiesUsesRsyncCheck(t *testing.T) {
	host := &config.ResolvedHost{}
	checks := RemoteChecksFromCapabilities(host, RemoteCapabilities{
		RsyncOK:         true,
		RsyncPath:       "/usr/bin/rsync",
		RsyncSource:     "path",
		SendrecvOK:      true,
		SendrecvPath:    "/usr/local/bin/sendrecv",
		TarOK:           true,
		GzipOK:          true,
		RemoteDirOK:     true,
		RemoteTempDirOK: true,
	})
	if len(checks) == 0 || checks[0].Name != "remote_rsync" {
		t.Fatalf("unexpected checks: %+v", checks)
	}
	if checks[0].Status != "ok" {
		t.Fatalf("unexpected rsync check: %+v", checks[0])
	}
}
