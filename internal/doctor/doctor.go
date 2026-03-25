package doctor

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"sendrecv/internal/config"
	"sendrecv/internal/remote"
)

type Check struct {
	Name   string
	Status string
	Detail string
}

type RemoteCapabilities struct {
	RsyncOK         bool
	SendrecvOK      bool
	TarOK           bool
	GzipOK          bool
	RemoteDirOK     bool
	RemoteTempDirOK bool
}

func LocalChecks(cfg *config.Config) []Check {
	tools := []struct {
		name string
		path string
	}{
		{name: "ssh", path: cfg.Tools.SSH},
		{name: "rsync", path: cfg.Tools.RSync},
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

func RemoteChecks(ctx context.Context, cfg *config.Config, host *config.ResolvedHost) []Check {
	capabilities, err := ProbeRemoteCapabilities(ctx, cfg, host)
	if err != nil {
		return []Check{{Name: "remote_probe", Status: "warning", Detail: fmt.Sprintf("remote capability probe failed: %v", err)}}
	}
	return []Check{
		statusCheck("remote_rsync", capabilities.RsyncOK, "remote rsync is available", "remote rsync is missing; transfers will fail"),
		statusCheck("remote_sendrecv", capabilities.SendrecvOK, "remote sendrecv is available for archive extract/pack", "remote sendrecv is missing; archive recv cannot use remote pack"),
		statusCheck("remote_tar", capabilities.TarOK, "remote tar is available", "remote tar is missing; shell extract fallback cannot run"),
		statusCheck("remote_gzip", capabilities.GzipOK, "remote gzip is available", "remote gzip is missing; shell extract fallback cannot run"),
		statusCheck("remote_dir", capabilities.RemoteDirOK, "remote_dir exists or can be created", "remote_dir is not writable or could not be created"),
		statusCheck("remote_temp_dir", capabilities.RemoteTempDirOK, "remote_temp_dir exists or can be created", "remote_temp_dir is not writable or could not be created"),
	}
}

func ProbeRemoteCapabilities(ctx context.Context, cfg *config.Config, host *config.ResolvedHost) (RemoteCapabilities, error) {
	command := strings.Join([]string{
		"printf 'rsync='",
		remote.CheckCommandStatus("rsync"),
		"printf '\\nsendrecv='",
		remote.CheckCommandStatus(host.SendrecvPath),
		"printf '\\ntar='",
		remote.CheckCommandStatus("tar"),
		"printf '\\ngzip='",
		remote.CheckCommandStatus("gzip"),
		"printf '\\nremote_dir='",
		remote.CheckMkdirStatus(host.RemoteDir),
		"printf '\\nremote_temp_dir='",
		remote.CheckMkdirStatus(host.RemoteTempDir),
	}, "; ")
	args := append(append([]string{}, host.SSHArgs...), host.SSHTarget, command)
	cmd := exec.CommandContext(ctx, cfg.Tools.SSH, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return RemoteCapabilities{}, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return parseRemoteCapabilities(string(output)), nil
}

func parseRemoteCapabilities(output string) RemoteCapabilities {
	var capabilities RemoteCapabilities
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		state := value == "ok"
		switch key {
		case "rsync":
			capabilities.RsyncOK = state
		case "sendrecv":
			capabilities.SendrecvOK = state
		case "tar":
			capabilities.TarOK = state
		case "gzip":
			capabilities.GzipOK = state
		case "remote_dir":
			capabilities.RemoteDirOK = state
		case "remote_temp_dir":
			capabilities.RemoteTempDirOK = state
		}
	}
	return capabilities
}

func statusCheck(name string, ok bool, okDetail string, failDetail string) Check {
	if ok {
		return Check{Name: name, Status: "ok", Detail: okDetail}
	}
	return Check{Name: name, Status: "warning", Detail: failDetail}
}
