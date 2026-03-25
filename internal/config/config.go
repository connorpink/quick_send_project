package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
}

type Host struct {
	SSHTarget     string   `toml:"ssh_target"`
	SendrecvPath  string   `toml:"sendrecv_path"`
	RemoteDir     string   `toml:"remote_dir"`
	RemoteTempDir string   `toml:"remote_temp_dir"`
	Extract       *bool    `toml:"extract"`
	RsyncArgs     []string `toml:"rsync_args"`
	SSHArgs       []string `toml:"ssh_args"`
}

type ResolvedHost struct {
	Name          string
	SSHTarget     string
	SendrecvPath  string
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
	return DefaultConfig().Render()
}

func DefaultConfig() *Config {
	return &Config{
		Defaults: Defaults{
			Extract:       true,
			Compression:   "gzip",
			RemoteTempDir: DefaultRemoteTempDir,
			RsyncArgs:     []string{"--archive", "--partial"},
			SSHArgs:       []string{"-o", "BatchMode=yes"},
		},
		Tools: Tools{
			SSH:   "ssh",
			RSync: "rsync",
		},
		Hosts: map[string]*Host{
			"laptop": {
				SSHTarget:     "user@laptop",
				SendrecvPath:  "sendrecv",
				RemoteDir:     "/home/user/Incoming",
				RemoteTempDir: DefaultRemoteTempDir,
			},
			"media": {
				SSHTarget:    "user@media-box",
				SendrecvPath: "/usr/local/bin/sendrecv",
				RemoteDir:    "/srv/incoming",
				Extract:      boolPtr(true),
				RsyncArgs:    []string{"--archive", "--partial", "--info=progress2"},
			},
		},
	}
}

func MinimalConfig() *Config {
	return &Config{
		Defaults: Defaults{
			Extract:       true,
			Compression:   "gzip",
			RemoteTempDir: DefaultRemoteTempDir,
			RsyncArgs:     []string{"--archive", "--partial"},
			SSHArgs:       []string{"-o", "BatchMode=yes"},
		},
		Tools: Tools{
			SSH:   "ssh",
			RSync: "rsync",
		},
		Hosts: map[string]*Host{},
	}
}

func (c *Config) Render() string {
	normalized := *c
	normalized.applyDefaults()

	var b strings.Builder
	b.WriteString("# sendrecv configuration\n")
	fmt.Fprintf(&b, "[defaults]\n")
	fmt.Fprintf(&b, "extract = %t\n", normalized.Defaults.Extract)
	fmt.Fprintf(&b, "compression = %s\n", quoteString(normalized.Defaults.Compression))
	fmt.Fprintf(&b, "remote_temp_dir = %s\n", quoteString(normalized.Defaults.RemoteTempDir))
	fmt.Fprintf(&b, "rsync_args = %s\n", renderStringList(normalized.Defaults.RsyncArgs))
	fmt.Fprintf(&b, "ssh_args = %s\n\n", renderStringList(normalized.Defaults.SSHArgs))

	fmt.Fprintf(&b, "[tools]\n")
	fmt.Fprintf(&b, "ssh = %s\n", quoteString(normalized.Tools.SSH))
	fmt.Fprintf(&b, "rsync = %s\n", quoteString(normalized.Tools.RSync))

	if len(normalized.Hosts) == 0 {
		b.WriteString("\n# Add host entries like:\n")
		b.WriteString("# [hosts.laptop]\n")
		b.WriteString("# ssh_target = \"user@laptop\"\n")
		b.WriteString("# sendrecv_path = \"sendrecv\"\n")
		b.WriteString("# remote_dir = \"/home/user/Incoming\"\n")
		b.WriteString("# remote_temp_dir = \"/home/user/Incoming/tmp\"\n")
		return b.String()
	}

	names := make([]string, 0, len(normalized.Hosts))
	for name := range normalized.Hosts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		host := normalized.Hosts[name]
		if host == nil {
			continue
		}
		fmt.Fprintf(&b, "\n[hosts.%s]\n", quoteKey(name))
		fmt.Fprintf(&b, "ssh_target = %s\n", quoteString(host.SSHTarget))
		if host.SendrecvPath != "" {
			fmt.Fprintf(&b, "sendrecv_path = %s\n", quoteString(host.SendrecvPath))
		}
		fmt.Fprintf(&b, "remote_dir = %s\n", quoteString(host.RemoteDir))
		if host.RemoteTempDir != "" {
			fmt.Fprintf(&b, "remote_temp_dir = %s\n", quoteString(host.RemoteTempDir))
		}
		if host.Extract != nil {
			fmt.Fprintf(&b, "extract = %t\n", *host.Extract)
		}
		if len(host.RsyncArgs) > 0 {
			fmt.Fprintf(&b, "rsync_args = %s\n", renderStringList(host.RsyncArgs))
		}
		if len(host.SSHArgs) > 0 {
			fmt.Fprintf(&b, "ssh_args = %s\n", renderStringList(host.SSHArgs))
		}
	}
	return b.String()
}

func Write(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(cfg.Render()), 0o644)
}

func boolPtr(value bool) *bool {
	return &value
}

func quoteString(value string) string {
	return strconv.Quote(value)
}

func quoteKey(value string) string {
	if value != "" && !strings.ContainsAny(value, ".-@/ \t\n\"'") {
		return value
	}
	return quoteString(value)
}

func renderStringList(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, quoteString(value))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	decoder := toml.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) WriteExample(path string) error {
	return Write(path, DefaultConfig())
}

func (c *Config) Validate() error {
	var errs []error
	if len(c.Hosts) == 0 {
		errs = append(errs, errors.New("config must define at least one host"))
	}
	if c.Defaults.Compression == "" {
		errs = append(errs, errors.New("defaults.compression is required"))
	}
	if c.Defaults.Compression != "gzip" {
		errs = append(errs, fmt.Errorf("unsupported defaults.compression %q", c.Defaults.Compression))
	}
	if err := validateTool("tools.ssh", c.Tools.SSH); err != nil {
		errs = append(errs, err)
	}
	if err := validateTool("tools.rsync", c.Tools.RSync); err != nil {
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
		if host.SendrecvPath != "" && validateCommandPath(host.SendrecvPath) != nil {
			errs = append(errs, fmt.Errorf("host %q sendrecv_path must be a bare executable name or absolute path", name))
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
		SendrecvPath:  firstNonEmpty(host.SendrecvPath, "sendrecv"),
		RemoteDir:     host.RemoteDir,
		RemoteTempDir: remoteTemp,
		Extract:       extract,
		RsyncArgs:     append(append([]string{}, c.Defaults.RsyncArgs...), host.RsyncArgs...),
		SSHArgs:       append(append([]string{}, c.Defaults.SSHArgs...), host.SSHArgs...),
	}, nil
}

func (c *Config) applyDefaults() {
	if c.Defaults.Compression == "" {
		c.Defaults.Compression = "gzip"
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
	if c.Defaults.RsyncArgs == nil {
		c.Defaults.RsyncArgs = []string{"--archive", "--partial"}
	}
	if c.Defaults.SSHArgs == nil {
		c.Defaults.SSHArgs = []string{"-o", "BatchMode=yes"}
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

func validateCommandPath(value string) error {
	if value == "" {
		return nil
	}
	if strings.ContainsAny(value, " \t\n") && !filepath.IsAbs(value) {
		return fmt.Errorf("invalid command path")
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
