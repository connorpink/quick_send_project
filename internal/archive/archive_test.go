package archive

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDecideIncompressibleSingleFileIsRaw(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "video.mp4")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	decision, err := Decide([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	if decision.Mode != ModeRaw {
		t.Fatalf("expected raw mode, got %+v", decision)
	}
}

func TestDecideDirectoryUsesArchive(t *testing.T) {
	dir := t.TempDir()
	decision, err := Decide([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if decision.Mode != ModeArchive || !decision.Compressed {
		t.Fatalf("expected archive mode, got %+v", decision)
	}
}
