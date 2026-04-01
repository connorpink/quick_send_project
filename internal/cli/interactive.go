package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"sort"
	"strings"

	"sendrecv/internal/config"
	"sendrecv/internal/doctor"
	"sendrecv/internal/picker"
	"sendrecv/internal/sshconfig"
)

func selectSendHost(ctx context.Context, cfg *config.Config, hostName string, opts *rootOptions, stderr io.Writer) (*config.ResolvedHost, error) {
	if hostName != "" {
		return cfg.ResolveHost(hostName)
	}
	switch len(cfg.Hosts) {
	case 0:
		return nil, fmt.Errorf("config has no hosts; run `sendrecv config init` or pass --config")
	case 1:
		for name := range cfg.Hosts {
			return cfg.ResolveHost(name)
		}
	}
	if !picker.IsInteractive() {
		return nil, fmt.Errorf("multiple hosts configured; pass --remote-host when not running in a terminal")
	}
	names := make([]string, 0, len(cfg.Hosts))
	for name := range cfg.Hosts {
		names = append(names, name)
	}
	sort.Strings(names)
	items := make([]picker.Item, 0, len(cfg.Hosts))
	for _, name := range names {
		host := cfg.Hosts[name]
		items = append(items, picker.Item{
			ID:    name,
			Label: name,
			Details: []string{
				host.SSHTarget,
				host.RemoteDir,
			},
		})
	}
	selected, err := picker.SelectOne(ctx, picker.Options{
		Prompt:  "sendrecv host> ",
		ForceGo: opts.GoFuzzy,
		Stderr:  stderr,
	}, items)
	if err != nil {
		if errors.Is(err, picker.ErrCancelled) {
			return nil, fmt.Errorf("host selection cancelled")
		}
		return nil, err
	}
	return cfg.ResolveHost(selected)
}

func runConfigInit(cmdIO io.Writer, errIO io.Writer, opts *rootOptions) error {
	configPath, err := resolvedConfigPath(opts.ConfigPath)
	if err != nil {
		return err
	}

	imported, skipped, err := loadSSHImportOptions()
	if err != nil {
		if err := config.Write(configPath, config.MinimalConfig()); err != nil {
			return err
		}
		fmt.Fprintf(cmdIO, "wrote %s\n", configPath)
		fmt.Fprintf(errIO, "ssh config import unavailable: %v\n", err)
		fmt.Fprintln(errIO, "fill in your hosts manually and then run `sendrecv config validate`.")
		return nil
	}
	for _, skip := range skipped {
		fmt.Fprintf(errIO, "ssh config: %s\n", skip)
	}
	if len(imported) == 0 {
		if err := config.Write(configPath, config.MinimalConfig()); err != nil {
			return err
		}
		fmt.Fprintf(cmdIO, "wrote %s\n", configPath)
		fmt.Fprintln(errIO, "no importable SSH hosts found; fill in your hosts manually.")
		return nil
	}
	if !picker.IsInteractive() {
		if err := config.Write(configPath, config.MinimalConfig()); err != nil {
			return err
		}
		fmt.Fprintf(cmdIO, "wrote %s\n", configPath)
		fmt.Fprintln(errIO, "interactive host import requires a terminal; fill in your hosts manually.")
		return nil
	}

	items := make([]picker.Item, 0, len(imported))
	byAlias := make(map[string]sshconfig.Host, len(imported))
	for _, host := range imported {
		byAlias[host.Alias] = host
		items = append(items, picker.Item{
			ID:    host.Alias,
			Label: host.Alias,
			Details: []string{
				host.User + "@" + host.HostName,
			},
		})
	}
	selected, err := picker.Select(context.Background(), picker.Options{
		Prompt:  "import ssh hosts> ",
		Multi:   true,
		ForceGo: opts.GoFuzzy,
		Stderr:  errIO,
	}, items)
	if err != nil {
		if errors.Is(err, picker.ErrCancelled) {
			selected = nil
		} else {
			return err
		}
	}
	if len(selected) == 0 {
		if err := config.Write(configPath, config.MinimalConfig()); err != nil {
			return err
		}
		fmt.Fprintf(cmdIO, "wrote %s\n", configPath)
		fmt.Fprintln(errIO, "no SSH hosts selected; fill in your hosts manually.")
		return nil
	}

	reader := bufio.NewReader(os.Stdin)
	cfg := config.MinimalConfig()
	cfg.Hosts = make(map[string]*config.Host, len(selected))
	for _, alias := range selected {
		host := byAlias[alias]
		remoteDir, err := promptAbsolutePath(reader, errIO, fmt.Sprintf("remote_dir for %s", alias))
		if err != nil {
			return err
		}
		cfg.Hosts[alias] = &config.Host{
			SSHTarget:     host.User + "@" + host.HostName,
			SendrecvPath:  "sendrecv",
			RemoteDir:     remoteDir,
			RemoteTempDir: pathpkg.Join(remoteDir, "tmp"),
		}
	}

	for _, alias := range selected {
		resolved, err := cfg.ResolveHost(alias)
		if err != nil {
			continue
		}
		capabilities, err := doctor.ProbeRemoteCapabilities(context.Background(), cfg, resolved)
		if err != nil {
			fmt.Fprintf(errIO, "%s/%s\t%s\t%s\n", alias, "remote_probe", "warning", fmt.Sprintf("remote capability probe failed: %v", err))
			continue
		}
		if capabilities.RsyncSource == "fallback" && resolved.RemoteRsyncPath == "" {
			cfg.Hosts[alias].RemoteRsyncPath = capabilities.RsyncPath
			resolved.RemoteRsyncPath = capabilities.RsyncPath
		}
		checks := doctor.RemoteChecksFromCapabilities(resolved, capabilities)
		for _, check := range checks {
			if check.Status == "ok" {
				continue
			}
			fmt.Fprintf(errIO, "%s\t%s\t%s\n", alias+"/"+check.Name, check.Status, check.Detail)
		}
	}

	if err := config.Write(configPath, cfg); err != nil {
		return err
	}
	fmt.Fprintf(cmdIO, "wrote %s\n", configPath)
	return nil
}

func loadSSHImportOptions() ([]sshconfig.Host, []string, error) {
	path, err := sshconfig.DefaultPath()
	if err != nil {
		return nil, nil, err
	}
	return sshconfig.Load(path)
}

func promptAbsolutePath(reader *bufio.Reader, out io.Writer, label string) (string, error) {
	for {
		fmt.Fprintf(out, "%s: ", label)
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		value := strings.TrimSpace(line)
		if value == "" {
			if errors.Is(err, io.EOF) {
				return "", fmt.Errorf("input ended before %s was provided", label)
			}
			fmt.Fprintln(out, "value is required")
			continue
		}
		if !pathpkg.IsAbs(value) {
			fmt.Fprintln(out, "value must be an absolute path")
			if errors.Is(err, io.EOF) {
				return "", fmt.Errorf("%s must be absolute", label)
			}
			continue
		}
		return value, nil
	}
}
