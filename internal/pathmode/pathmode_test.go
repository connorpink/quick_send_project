package pathmode

import (
	"path/filepath"
	"testing"
)

func TestBuildMappingsStripCommonPrefix(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a", "one.txt")
	b := filepath.Join(root, "a", "nested", "two.txt")
	mappings, base, err := BuildMappings([]string{a, b}, StripCommonPrefix)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(base) != filepath.Join(root, "a") {
		t.Fatalf("unexpected base: %s", base)
	}
	if mappings[0].Target != "one.txt" {
		t.Fatalf("unexpected target: %s", mappings[0].Target)
	}
	if mappings[1].Target != "nested/two.txt" {
		t.Fatalf("unexpected target: %s", mappings[1].Target)
	}
}
