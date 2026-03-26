package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	SendrecvPath    string
	TarOK           bool
	GzipOK          bool
	RemoteDirOK     bool
	RemoteTempDirOK bool
}

func LocalChecks(configPath string) []Check {
	checks := []Check{ConfigCheck(configPath)}
	cfg, err := config.Load(configPath)
	if err != nil {
		checks = append(checks, Check{Name: "config_parse", Status: "warning", Detail: err.Error()})
	} else {
		checks = append(checks, Check{Name: "config_parse", Status: "ok", Detail: "config is valid"})
		checks = append(checks, HostCountCheck(cfg))
	}

	tools := []struct {
		name string
		path string
	}{
		{name: "ssh", path: toolPath(cfg, "ssh")},
		{name: "rsync", path: toolPath(cfg, "rsync")},
		{name: "fzf", path: "fzf"},
	}
	for _, tool := range tools {
		_, err := exec.LookPath(tool.path)
		status := "ok"
		detail := "found"
		if err != nil {
			status = "warning"
			if tool.name == "fzf" {
				detail = "not found; Go fuzzy picker fallback will be used"
			} else {
				detail = err.Error()
			}
		}
		checks = append(checks, Check{Name: tool.name, Status: status, Detail: detail})
	}
	return checks
}

func ConfigCheck(path string) Check {
	info, err := os.Stat(path)
	if err == nil && !info.IsDir() {
		return Check{Name: "config_path", Status: "ok", Detail: path}
	}
	if err != nil && os.IsNotExist(err) {
		return Check{Name: "config_path", Status: "warning", Detail: fmt.Sprintf("missing config at %s; run `sendrecv config init`", path)}
	}
	if err != nil {
		return Check{Name: "config_path", Status: "warning", Detail: err.Error()}
	}
	return Check{Name: "config_path", Status: "warning", Detail: fmt.Sprintf("%s is a directory", path)}
}

func HostCountCheck(cfg *config.Config) Check {
	count := len(cfg.Hosts)
	if count == 0 {
		return Check{Name: "config_hosts", Status: "warning", Detail: "config has no hosts; `sendrecv send` will not work until you add one"}
	}
	return Check{Name: "config_hosts", Status: "ok", Detail: fmt.Sprintf("%d host(s) configured", count)}
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
		"printf '\\nsendrecv_path='",
		remote.ResolveSendrecvPathCommand(host.SendrecvPath),
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
		switch key {
		case "rsync":
			capabilities.RsyncOK = value == "ok"
		case "sendrecv_path":
			capabilities.SendrecvOK = value != "" && value != "missing"
			if capabilities.SendrecvOK {
				capabilities.SendrecvPath = value
			}
		case "tar":
			capabilities.TarOK = value == "ok"
		case "gzip":
			capabilities.GzipOK = value == "ok"
		case "remote_dir":
			capabilities.RemoteDirOK = value == "ok"
		case "remote_temp_dir":
			capabilities.RemoteTempDirOK = value == "ok"
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

func YaziChecks() []Check {
	keymapPaths, err := yaziKeymapPaths()
	if err != nil {
		return []Check{{Name: "yazi_keymap", Status: "warning", Detail: err.Error()}}
	}
	pluginPaths, err := yaziPluginPaths("sendrecv.yazi")
	if err != nil {
		return []Check{{Name: "yazi_plugin", Status: "warning", Detail: err.Error()}}
	}

	checks := make([]Check, 0, len(keymapPaths)+len(pluginPaths))
	for _, keymapPath := range keymapPaths {
		info, err := os.Stat(keymapPath)
		if err == nil && !info.IsDir() {
			checks = append(checks, Check{Name: "yazi_keymap", Status: "ok", Detail: fmt.Sprintf("found %s", keymapPath)})
			continue
		}
		if err != nil && os.IsNotExist(err) {
			checks = append(checks, Check{Name: "yazi_keymap", Status: "warning", Detail: fmt.Sprintf("missing %s", keymapPath)})
			continue
		}
		if err != nil {
			checks = append(checks, Check{Name: "yazi_keymap", Status: "warning", Detail: err.Error()})
			continue
		}
		checks = append(checks, Check{Name: "yazi_keymap", Status: "warning", Detail: fmt.Sprintf("%s is a directory", keymapPath)})
	}
	for _, pluginPath := range pluginPaths {
		info, err := os.Stat(pluginPath)
		if err == nil && !info.IsDir() {
			checks = append(checks, Check{Name: "yazi_plugin", Status: "ok", Detail: fmt.Sprintf("found %s", pluginPath)})
			continue
		}
		if err != nil && os.IsNotExist(err) {
			checks = append(checks, Check{Name: "yazi_plugin", Status: "warning", Detail: fmt.Sprintf("missing %s", pluginPath)})
			continue
		}
		if err != nil {
			checks = append(checks, Check{Name: "yazi_plugin", Status: "warning", Detail: err.Error()})
			continue
		}
		checks = append(checks, Check{Name: "yazi_plugin", Status: "warning", Detail: fmt.Sprintf("%s is a directory", pluginPath)})
	}
	return checks
}

func toolPath(cfg *config.Config, tool string) string {
	if cfg == nil {
		switch tool {
		case "ssh":
			return "ssh"
		case "rsync":
			return "rsync"
		}
		return tool
	}
	switch tool {
	case "ssh":
		return cfg.Tools.SSH
	case "rsync":
		return cfg.Tools.RSync
	default:
		return tool
	}
}

func yaziKeymapPaths() ([]string, error) {
	baseDirs, err := yaziConfigDirs()
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(baseDirs))
	for _, baseDir := range baseDirs {
		paths = append(paths, filepath.Join(baseDir, "keymap.toml"))
	}
	return paths, nil
}

func yaziPluginPaths(pluginDir string) ([]string, error) {
	baseDirs, err := yaziConfigDirs()
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(baseDirs))
	for _, baseDir := range baseDirs {
		paths = append(paths, filepath.Join(baseDir, "plugins", pluginDir, "main.lua"))
	}
	return paths, nil
}

func yaziConfigDirs() ([]string, error) {
	paths := make([]string, 0, 2)
	cfgDir, err := os.UserConfigDir()
	if err == nil {
		paths = append(paths, filepath.Join(cfgDir, "yazi"))
	}

	home, homeErr := os.UserHomeDir()
	if homeErr != nil {
		if len(paths) > 0 {
			return paths, nil
		}
		return nil, homeErr
	}

	xdgPath := filepath.Join(home, ".config", "yazi")
	for _, existing := range paths {
		if existing == xdgPath {
			return paths, nil
		}
	}
	paths = append(paths, xdgPath)
	return paths, nil
}
