package archive

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
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
	".zst":  {},
	".tar":  {},
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
	if IsLikelyIncompressible(paths[0]) {
		return Decision{Mode: ModeRaw, Compressed: false, Reason: "single incompressible file can transfer raw"}, nil
	}
	return Decision{Mode: ModeArchive, Compressed: true, Reason: "single compressible file benefits from gzip"}, nil
}

func IsLikelyIncompressible(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	_, ok := incompressibleExt[ext]
	return ok
}

func CreateTarGz(baseDir, outputPath string, members []string) error {
	if len(members) == 0 {
		return errors.New("archive requires at least one member")
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzw := gzip.NewWriter(file)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	for _, member := range members {
		cleanMember, err := validateMemberPath(member)
		if err != nil {
			return err
		}
		sourcePath := filepath.Join(baseDir, filepath.FromSlash(cleanMember))
		if err := addPathToArchive(tw, sourcePath, cleanMember); err != nil {
			return err
		}
	}
	return nil
}

func ExtractTarGz(archivePath, destination string) error {
	if !filepath.IsAbs(destination) {
		return fmt.Errorf("destination must be absolute: %s", destination)
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return err
	}
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		targetPath, err := resolveExtractPath(destination, header.Name)
		if err != nil {
			return err
		}
		if err := extractHeader(tr, header, targetPath); err != nil {
			return err
		}
	}
}

func addPathToArchive(tw *tar.Writer, sourcePath, archiveName string) error {
	return filepath.Walk(sourcePath, func(current string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(sourcePath, current)
		if err != nil {
			return err
		}
		name := archiveName
		if rel != "." {
			name = filepath.ToSlash(filepath.Join(archiveName, rel))
		}
		linkTarget := ""
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err = os.Readlink(current)
			if err != nil {
				return err
			}
		}
		header, err := tar.FileInfoHeader(info, linkTarget)
		if err != nil {
			return err
		}
		header.Name = name
		if info.IsDir() && !strings.HasSuffix(header.Name, "/") {
			header.Name += "/"
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		file, err := os.Open(current)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(tw, file)
		return err
	})
}

func resolveExtractPath(destination, name string) (string, error) {
	cleanName, err := validateMemberPath(name)
	if err != nil {
		return "", err
	}
	targetPath := filepath.Join(destination, filepath.FromSlash(cleanName))
	cleanDest := filepath.Clean(destination)
	cleanTarget := filepath.Clean(targetPath)
	if cleanTarget != cleanDest && !strings.HasPrefix(cleanTarget, cleanDest+string(filepath.Separator)) {
		return "", fmt.Errorf("archive entry escapes destination: %s", name)
	}
	return cleanTarget, nil
}

func validateMemberPath(name string) (string, error) {
	if name == "" {
		return "", errors.New("archive member cannot be empty")
	}
	clean := path.Clean(strings.ReplaceAll(name, "\\", "/"))
	if clean == "." {
		return "", errors.New("archive member cannot resolve to current directory")
	}
	if strings.HasPrefix(clean, "/") {
		return "", fmt.Errorf("archive member must be relative: %s", name)
	}
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("archive member escapes base directory: %s", name)
	}
	return clean, nil
}

func extractHeader(tr *tar.Reader, header *tar.Header, targetPath string) error {
	mode := os.FileMode(header.Mode)
	switch header.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(targetPath, mode.Perm())
	case tar.TypeReg, tar.TypeRegA:
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode.Perm())
		if err != nil {
			return err
		}
		defer file.Close()
		if _, err := io.Copy(file, tr); err != nil {
			return err
		}
		return nil
	case tar.TypeSymlink:
		if filepath.IsAbs(header.Linkname) {
			return fmt.Errorf("absolute symlink target is not allowed: %s", header.Linkname)
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		_ = os.Remove(targetPath)
		return os.Symlink(header.Linkname, targetPath)
	default:
		return fmt.Errorf("unsupported archive entry type %q for %s", string(header.Typeflag), header.Name)
	}
}
