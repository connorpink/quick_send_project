package doctor

import (
	"context"
	"fmt"
	"os/exec"

	"sendrecv/internal/config"
)

type Check struct {
	Name   string
	Status string
	Detail string
}

func LocalChecks(cfg *config.Config) []Check {
	tools := []struct {
		name string
		path string
	}{
		{name: "ssh", path: cfg.Tools.SSH},
		{name: "rsync", path: cfg.Tools.RSync},
		{name: "tar", path: cfg.Tools.Tar},
		{name: "xz", path: cfg.Tools.XZ},
	}
	checks := make([]Check, 0, len(tools))
	for _, tool := range tools {
		_, err := exec.LookPath(tool.path)
		status := "ok"
		detail := "found"
		if err != nil {
			status = "missing"
			detail = err.Error()
		}
		checks = append(checks, Check{Name: tool.name, Status: status, Detail: detail})
	}
	return checks
}

func RemoteCheck(ctx context.Context, cfg *config.Config, host *config.ResolvedHost) Check {
	args := append(append([]string{}, host.SSHArgs...), host.SSHTarget, "command -v tar >/dev/null && command -v xz >/dev/null && test -d "+host.RemoteDir)
	cmd := exec.CommandContext(ctx, cfg.Tools.SSH, args...)
	if err := cmd.Run(); err != nil {
		return Check{Name: "remote", Status: "warning", Detail: fmt.Sprintf("remote checks failed: %v", err)}
	}
	return Check{Name: "remote", Status: "ok", Detail: "remote tar/xz and destination path available"}
}
