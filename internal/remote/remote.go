package remote

import (
	"fmt"
	"path"
	"strings"
)

func ArchivePath(tempDir string) string {
	return path.Join(tempDir, "sendrecv-transfer.tar.xz")
}

func Quote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func ExtractCommand(archivePath, destination string, keepArchive bool) string {
	cmd := fmt.Sprintf("mkdir -p %s && xz -dc %s | tar -xf - -C %s", Quote(destination), Quote(archivePath), Quote(destination))
	if keepArchive {
		return cmd
	}
	return cmd + " && rm -f " + Quote(archivePath)
}

func CreateArchiveCommand(baseDir, archivePath string, members []string) string {
	quoted := make([]string, 0, len(members))
	for _, m := range members {
		quoted = append(quoted, Quote(m))
	}
	return fmt.Sprintf("mkdir -p %s && tar -C %s -cf - %s | xz -zc > %s",
		Quote(path.Dir(archivePath)),
		Quote(baseDir),
		strings.Join(quoted, " "),
		Quote(archivePath),
	)
}
