package archive

import (
	"archive/tar"
	"compress/gzip"
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

func TestCreateAndExtractTarGzRoundTrip(t *testing.T) {
	base := t.TempDir()
	sourceDir := filepath.Join(base, "project")
	if err := os.MkdirAll(filepath.Join(sourceDir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "nested", "file.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(t.TempDir(), "project.tar.gz")
	if err := CreateTarGz(base, archivePath, []string{"project"}); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	if err := ExtractTarGz(archivePath, dest); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dest, "project", "nested", "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Fatalf("unexpected file contents: %q", data)
	}
}

func TestExtractTarGzRejectsTraversal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unsafe.tar.gz")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gzw := gzip.NewWriter(file)
	tw := tar.NewWriter(gzw)
	if err := tw.WriteHeader(&tar.Header{Name: "../escape.txt", Mode: 0o644, Size: 4}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("oops")); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := ExtractTarGz(path, t.TempDir()); err == nil {
		t.Fatal("expected traversal archive to fail")
	}
}
