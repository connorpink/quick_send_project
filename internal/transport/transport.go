package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"sendrecv/internal/archive"
	"sendrecv/internal/config"
	"sendrecv/internal/pathmode"
	"sendrecv/internal/remote"
)

type ExecOptions struct {
	DryRun  bool
	Verbose bool
	Stdout  io.Writer
	Stderr  io.Writer
}

type TransferOptions struct {
	Extract      *bool
	KeepArchive  bool
	PreserveTree bool
}

type Plan struct {
	Summary  string
	Commands []Command
}

type Command struct {
	Name string
	Args []string
}

func (c Command) String() string {
	parts := make([]string, 0, len(c.Args)+1)
	parts = append(parts, shellQuote(c.Name))
	for _, arg := range c.Args {
		parts = append(parts, shellQuote(arg))
	}
	return strings.Join(parts, " ")
}

type Runner struct {
	Config *config.Config
	Exec   ExecOptions
}

func (r Runner) SendPlan(host *config.ResolvedHost, paths []string, opts TransferOptions) (*Plan, error) {
	if len(paths) == 0 {
		return nil, errors.New("send requires at least one path")
	}
	mode := pathmode.StripCommonPrefix
	if opts.PreserveTree {
		mode = pathmode.PreserveTree
	}
	mappings, base, err := pathmode.BuildMappings(paths, mode)
	if err != nil {
		return nil, err
	}
	decision, err := archive.Decide(paths)
	if err != nil {
		return nil, err
	}
	extract := host.Extract
	if opts.Extract != nil {
		extract = *opts.Extract
	}
	if decision.Mode == archive.ModeRaw && !extract {
		return r.rawSendPlan(host, mappings), nil
	}
	if decision.Mode == archive.ModeRaw && extract {
		return r.rawSendPlan(host, mappings), nil
	}
	members := make([]string, 0, len(mappings))
	for _, m := range mappings {
		members = append(members, filepath.ToSlash(m.Target))
	}
	localArchive := filepath.Join(os.TempDir(), "sendrecv-transfer.tar.xz")
	remoteArchive := remote.ArchivePath(host.RemoteTempDir)
	commands := []Command{
		{Name: "sh", Args: []string{"-c", localArchiveCommand(r.Config, base, members, localArchive)}},
		{Name: r.Config.Tools.RSync, Args: rsyncArgs(r.Config, host, localArchive, host.SSHTarget+":"+remoteArchive)},
	}
	if extract {
		commands = append(commands, Command{
			Name: r.Config.Tools.SSH,
			Args: append(append([]string{}, host.SSHArgs...), host.SSHTarget, remote.ExtractCommand(remoteArchive, host.RemoteDir, opts.KeepArchive)),
		})
	}
	if !opts.KeepArchive {
		commands = append(commands, Command{Name: "rm", Args: []string{"-f", localArchive}})
	}
	return &Plan{
		Summary:  decision.Reason,
		Commands: commands,
	}, nil
}

func (r Runner) RecvPlan(host *config.ResolvedHost, paths []string, opts TransferOptions) (*Plan, error) {
	if len(paths) == 0 {
		return nil, errors.New("recv requires at least one path")
	}
	extract := host.Extract
	if opts.Extract != nil {
		extract = *opts.Extract
	}
	remoteArchive := remote.ArchivePath(host.RemoteTempDir)
	localArchive := filepath.Join(os.TempDir(), "sendrecv-transfer.tar.xz")
	remotePaths := make([]string, 0, len(paths))
	for _, p := range paths {
		if path.IsAbs(p) {
			remotePaths = append(remotePaths, p)
			continue
		}
		remotePaths = append(remotePaths, path.Join(host.RemoteDir, p))
	}
	base := commonRemoteBase(remotePaths)
	members := make([]string, 0, len(remotePaths))
	for _, p := range remotePaths {
		members = append(members, strings.TrimPrefix(strings.TrimPrefix(p, base), "/"))
	}
	commands := []Command{
		{
			Name: r.Config.Tools.SSH,
			Args: append(append([]string{}, host.SSHArgs...), host.SSHTarget, remote.CreateArchiveCommand(base, remoteArchive, members)),
		},
		{
			Name: r.Config.Tools.RSync,
			Args: rsyncArgs(r.Config, host, host.SSHTarget+":"+remoteArchive, localArchive),
		},
	}
	if extract {
		commands = append(commands, Command{Name: "sh", Args: []string{"-c", fmt.Sprintf("mkdir -p %s && xz -dc %s | tar -xf - -C %s", shellQuote("."), shellQuote(localArchive), shellQuote("."))}})
	}
	if !opts.KeepArchive {
		commands = append(commands, Command{Name: "rm", Args: []string{"-f", localArchive}})
	}
	if !opts.KeepArchive {
		commands = append(commands, Command{
			Name: r.Config.Tools.SSH,
			Args: append(append([]string{}, host.SSHArgs...), host.SSHTarget, "rm -f "+remote.Quote(remoteArchive)),
		})
	}
	return &Plan{
		Summary:  "receive remote files through archive pipeline",
		Commands: commands,
	}, nil
}

func (r Runner) rawSendPlan(host *config.ResolvedHost, mappings []pathmode.Mapping) *Plan {
	args := append([]string{}, host.RsyncArgs...)
	for _, mapping := range mappings {
		targetDir := path.Join(host.RemoteDir, path.Dir(filepath.ToSlash(mapping.Target)))
		args = rsyncArgs(r.Config, host, mapping.Source, host.SSHTarget+":"+targetDir+"/")
	}
	return &Plan{
		Summary:  "single incompressible file can transfer raw",
		Commands: []Command{{Name: r.Config.Tools.RSync, Args: args}},
	}
}

func (r Runner) Execute(ctx context.Context, plan *Plan) error {
	for _, command := range plan.Commands {
		if r.Exec.DryRun {
			fmt.Fprintln(r.Exec.Stdout, command.String())
			continue
		}
		if r.Exec.Verbose {
			fmt.Fprintln(r.Exec.Stdout, command.String())
		}
		cmd := exec.CommandContext(ctx, command.Name, command.Args...)
		cmd.Stdout = r.Exec.Stdout
		cmd.Stderr = r.Exec.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("command failed: %s: %w", command.String(), err)
		}
	}
	return nil
}

func commonRemoteBase(paths []string) string {
	if len(paths) == 1 {
		return path.Dir(paths[0])
	}
	parts := strings.Split(strings.TrimPrefix(path.Clean(paths[0]), "/"), "/")
	for _, p := range paths[1:] {
		current := strings.Split(strings.TrimPrefix(path.Clean(p), "/"), "/")
		max := min(len(parts), len(current))
		var i int
		for i = 0; i < max && parts[i] == current[i]; i++ {
		}
		parts = parts[:i]
	}
	return "/" + strings.Join(parts, "/")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	if !strings.ContainsAny(value, " \t\n'\"\\$&;()<>|*?[]{}") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func localArchiveCommand(cfg *config.Config, base string, members []string, archivePath string) string {
	quotedMembers := make([]string, 0, len(members))
	for _, member := range members {
		quotedMembers = append(quotedMembers, shellQuote(member))
	}
	return fmt.Sprintf("mkdir -p %s && %s -C %s -cf - %s | %s -zc > %s",
		shellQuote(filepath.Dir(archivePath)),
		shellQuote(cfg.Tools.Tar),
		shellQuote(base),
		strings.Join(quotedMembers, " "),
		shellQuote(cfg.Tools.XZ),
		shellQuote(archivePath),
	)
}

func rsyncArgs(cfg *config.Config, host *config.ResolvedHost, source, destination string) []string {
	args := append([]string{}, host.RsyncArgs...)
	args = append(args, "-e", strings.TrimSpace(strings.Join(append([]string{cfg.Tools.SSH}, host.SSHArgs...), " ")))
	args = append(args, source, destination)
	return args
}
