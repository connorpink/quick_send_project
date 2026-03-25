package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

const DefaultRemoteTempDir = "/tmp/sendrecv"

type Config struct {
	Defaults Defaults         `toml:"defaults"`
	Tools    Tools            `toml:"tools"`
	Hosts    map[string]*Host `toml:"hosts"`
}

type Defaults struct {
	Extract       bool     `toml:"extract"`
	Compression   string   `toml:"compression"`
	RemoteTempDir string   `toml:"remote_temp_dir"`
	RsyncArgs     []string `toml:"rsync_args"`
	SSHArgs       []string `toml:"ssh_args"`
}

type Tools struct {
	SSH   string `toml:"ssh"`
	RSync string `toml:"rsync"`
	Tar   string `toml:"tar"`
	XZ    string `toml:"xz"`
}

type Host struct {
	SSHTarget     string   `toml:"ssh_target"`
	RemoteDir     string   `toml:"remote_dir"`
	RemoteTempDir string   `toml:"remote_temp_dir"`
	Extract       *bool    `toml:"extract"`
	RsyncArgs     []string `toml:"rsync_args"`
	SSHArgs       []string `toml:"ssh_args"`
}

type ResolvedHost struct {
	Name          string
	SSHTarget     string
	RemoteDir     string
	RemoteTempDir string
	Extract       bool
	RsyncArgs     []string
	SSHArgs       []string
}

func DefaultPath() (string, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfgDir, "sendrecv", "config.toml"), nil
}

func Example() string {
	return `# sendrecv configuration
[defaults]
extract = true
compression = "xz"
remote_temp_dir = "/tmp/sendrecv"
rsync_args = ["--archive", "--partial"]
ssh_args = ["-o", "BatchMode=yes"]

[tools]
ssh = "ssh"
rsync = "rsync"
tar = "tar"
xz = "xz"

[hosts.laptop]
ssh_target = "user@laptop"
remote_dir = "/home/user/Incoming"
remote_temp_dir = "/tmp/sendrecv"

[hosts.media]
ssh_target = "user@media-box"
remote_dir = "/srv/incoming"
extract = true
rsync_args = ["--archive", "--partial", "--info=progress2"]
`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) WriteExample(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(Example()), 0o644)
}

func (c *Config) Validate() error {
	var errs []error
	if len(c.Hosts) == 0 {
		errs = append(errs, errors.New("config must define at least one host"))
	}
	if c.Defaults.Compression == "" {
		errs = append(errs, errors.New("defaults.compression is required"))
	}
	if c.Defaults.Compression != "xz" {
		errs = append(errs, fmt.Errorf("unsupported defaults.compression %q", c.Defaults.Compression))
	}
	if err := validateTool("tools.ssh", c.Tools.SSH); err != nil {
		errs = append(errs, err)
	}
	if err := validateTool("tools.rsync", c.Tools.RSync); err != nil {
		errs = append(errs, err)
	}
	if err := validateTool("tools.tar", c.Tools.Tar); err != nil {
		errs = append(errs, err)
	}
	if err := validateTool("tools.xz", c.Tools.XZ); err != nil {
		errs = append(errs, err)
	}

	for name, host := range c.Hosts {
		if strings.TrimSpace(name) == "" {
			errs = append(errs, errors.New("host name cannot be empty"))
			continue
		}
		if host == nil {
			errs = append(errs, fmt.Errorf("host %q is nil", name))
			continue
		}
		if strings.TrimSpace(host.SSHTarget) == "" {
			errs = append(errs, fmt.Errorf("host %q must set ssh_target", name))
		}
		if !filepath.IsAbs(host.RemoteDir) {
			errs = append(errs, fmt.Errorf("host %q remote_dir must be absolute", name))
		}
		if host.RemoteTempDir != "" && !filepath.IsAbs(host.RemoteTempDir) {
			errs = append(errs, fmt.Errorf("host %q remote_temp_dir must be absolute", name))
		}
	}

	return errors.Join(errs...)
}

func (c *Config) ResolveHost(name string) (*ResolvedHost, error) {
	host, ok := c.Hosts[name]
	if !ok {
		return nil, fmt.Errorf("unknown host %q", name)
	}
	extract := c.Defaults.Extract
	if host.Extract != nil {
		extract = *host.Extract
	}
	remoteTemp := c.Defaults.RemoteTempDir
	if host.RemoteTempDir != "" {
		remoteTemp = host.RemoteTempDir
	}
	return &ResolvedHost{
		Name:          name,
		SSHTarget:     host.SSHTarget,
		RemoteDir:     host.RemoteDir,
		RemoteTempDir: remoteTemp,
		Extract:       extract,
		RsyncArgs:     append(append([]string{}, c.Defaults.RsyncArgs...), host.RsyncArgs...),
		SSHArgs:       append(append([]string{}, c.Defaults.SSHArgs...), host.SSHArgs...),
	}, nil
}

func (c *Config) applyDefaults() {
	if c.Defaults.Compression == "" {
		c.Defaults.Compression = "xz"
	}
	if c.Defaults.RemoteTempDir == "" {
		c.Defaults.RemoteTempDir = DefaultRemoteTempDir
	}
	if c.Tools.SSH == "" {
		c.Tools.SSH = "ssh"
	}
	if c.Tools.RSync == "" {
		c.Tools.RSync = "rsync"
	}
	if c.Tools.Tar == "" {
		c.Tools.Tar = "tar"
	}
	if c.Tools.XZ == "" {
		c.Tools.XZ = "xz"
	}
	if c.Defaults.RsyncArgs == nil {
		c.Defaults.RsyncArgs = []string{"--archive", "--partial"}
	}
	if c.Defaults.SSHArgs == nil {
		c.Defaults.SSHArgs = []string{"-o", "BatchMode=yes"}
	}
	if runtime.GOOS == "darwin" && c.Defaults.RemoteTempDir == "" {
		c.Defaults.RemoteTempDir = DefaultRemoteTempDir
	}
}

func validateTool(name, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	if strings.ContainsAny(value, " \t\n") && !filepath.IsAbs(value) {
		return fmt.Errorf("%s must be a bare executable name or absolute path", name)
	}
	return nil
}
