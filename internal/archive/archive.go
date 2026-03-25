package archive

import (
	"os"
	"path/filepath"
	"strings"
)

type Mode int

const (
	ModeRaw Mode = iota
	ModeArchive
)

type Decision struct {
	Mode       Mode
	Compressed bool
	Reason     string
}

var incompressibleExt = map[string]struct{}{
	".7z":   {},
	".avi":  {},
	".gz":   {},
	".iso":  {},
	".jpg":  {},
	".jpeg": {},
	".m4v":  {},
	".mkv":  {},
	".mov":  {},
	".mp3":  {},
	".mp4":  {},
	".png":  {},
	".rar":  {},
	".tgz":  {},
	".xz":   {},
	".zip":  {},
}

func Decide(paths []string) (Decision, error) {
	if len(paths) != 1 {
		return Decision{Mode: ModeArchive, Compressed: true, Reason: "multiple paths require an archive"}, nil
	}
	info, err := os.Stat(paths[0])
	if err != nil {
		return Decision{}, err
	}
	if info.IsDir() {
		return Decision{Mode: ModeArchive, Compressed: true, Reason: "directories require an archive"}, nil
	}
	ext := strings.ToLower(filepath.Ext(paths[0]))
	if _, ok := incompressibleExt[ext]; ok {
		return Decision{Mode: ModeRaw, Compressed: false, Reason: "single incompressible file can transfer raw"}, nil
	}
	return Decision{Mode: ModeArchive, Compressed: true, Reason: "single compressible file benefits from xz"}, nil
}
