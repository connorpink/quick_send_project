package pathmode

import (
	"fmt"
	"path/filepath"
	"strings"
)

type Mode int

const (
	StripCommonPrefix Mode = iota
	PreserveTree
)

type Mapping struct {
	Source string
	Base   string
	Target string
}

func BuildMappings(paths []string, mode Mode) ([]Mapping, string, error) {
	if len(paths) == 0 {
		return nil, "", fmt.Errorf("no paths provided")
	}
	abs := make([]string, 0, len(paths))
	for _, p := range paths {
		a, err := filepath.Abs(p)
		if err != nil {
			return nil, "", err
		}
		abs = append(abs, filepath.Clean(a))
	}
	var base string
	if mode == PreserveTree {
		wd, err := filepath.Abs(".")
		if err != nil {
			return nil, "", err
		}
		base = wd
	} else {
		base = commonDir(abs)
	}

	mappings := make([]Mapping, 0, len(abs))
	for _, src := range abs {
		rel, err := filepath.Rel(base, src)
		if err != nil {
			return nil, "", err
		}
		mappings = append(mappings, Mapping{
			Source: src,
			Base:   base,
			Target: filepath.ToSlash(rel),
		})
	}
	return mappings, base, nil
}

func commonDir(paths []string) string {
	if len(paths) == 1 {
		return filepath.Dir(paths[0])
	}
	parts := splitPath(paths[0])
	for _, p := range paths[1:] {
		current := splitPath(p)
		max := min(len(parts), len(current))
		var i int
		for i = 0; i < max && parts[i] == current[i]; i++ {
		}
		parts = parts[:i]
	}
	if len(parts) == 0 {
		return string(filepath.Separator)
	}
	return filepath.Join(parts...)
}

func splitPath(value string) []string {
	clean := filepath.Clean(value)
	if clean == string(filepath.Separator) {
		return []string{string(filepath.Separator)}
	}
	if filepath.IsAbs(clean) {
		return append([]string{string(filepath.Separator)}, strings.Split(strings.TrimPrefix(clean, string(filepath.Separator)), string(filepath.Separator))...)
	}
	return strings.Split(clean, string(filepath.Separator))
}
