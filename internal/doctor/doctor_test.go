package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
