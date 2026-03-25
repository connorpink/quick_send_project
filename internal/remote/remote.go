package remote

import (
	"fmt"
	"path"
	"strings"
)

const ArchiveFileName = "sendrecv-transfer.tar.gz"

func ArchivePath(tempDir string) string {
	return path.Join(tempDir, ArchiveFileName)
}

func Quote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func MkdirCommand(dir string) string {
	return "mkdir -p " + Quote(dir)
}

func CleanupCommand(target string) string {
	return "rm -f " + Quote(target)
}

func UnpackCommand(sendrecvPath, archivePath, destination string, keepArchive bool) string {
	cmd := fmt.Sprintf("%s unpack --archive %s --dest %s", Quote(sendrecvPath), Quote(archivePath), Quote(destination))
	if keepArchive {
		return cmd
	}
	return cmd + " && " + CleanupCommand(archivePath)
}

func TarExtractCommand(archivePath, destination string, keepArchive bool) string {
	cmd := fmt.Sprintf("mkdir -p %s && gzip -dc %s | tar -xf - -C %s", Quote(destination), Quote(archivePath), Quote(destination))
	if keepArchive {
		return cmd
	}
	return cmd + " && " + CleanupCommand(archivePath)
}

func PackCommand(sendrecvPath, archivePath, baseDir string, members []string) string {
	quoted := make([]string, 0, len(members))
	for _, member := range members {
		quoted = append(quoted, Quote(member))
	}
	return fmt.Sprintf("%s pack --output %s --base %s %s",
		Quote(sendrecvPath),
		Quote(archivePath),
		Quote(baseDir),
		strings.Join(quoted, " "),
	)
}

func CheckBinaryCommand(sendrecvPath string) string {
	if strings.HasPrefix(sendrecvPath, "/") {
		return "test -x " + Quote(sendrecvPath)
	}
	return "command -v " + Quote(sendrecvPath) + " >/dev/null"
}

func CheckCommandStatus(command string) string {
	return fmt.Sprintf("if %s; then printf ok; else printf missing; fi",
		CheckBinaryCommand(command),
	)
}

func CheckMkdirStatus(dir string) string {
	return fmt.Sprintf("if mkdir -p %s >/dev/null 2>&1; then printf ok; else printf missing; fi", Quote(dir))
}
