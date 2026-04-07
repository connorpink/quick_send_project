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
	"sendrecv/internal/doctor"
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
	SendMode     *config.SendTransferMode
}

type Plan struct {
	Summary    string
	Operations []Operation
}

type Operation interface {
	Display() string
	Run(context.Context, Runner) error
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

type CommandOperation struct {
	Command Command
}

func (o CommandOperation) Display() string {
	return o.Command.String()
}

func (o CommandOperation) Run(ctx context.Context, runner Runner) error {
	cmd := exec.CommandContext(ctx, o.Command.Name, o.Command.Args...)
	cmd.Stdout = runner.Exec.Stdout
	cmd.Stderr = runner.Exec.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed: %s: %w", o.Command.String(), err)
	}
	return nil
}

type PackOperation struct {
	BaseDir    string
	OutputPath string
	Members    []string
}

func (o PackOperation) Display() string {
	command := Command{
		Name: "sendrecv",
		Args: append([]string{"pack", "--output", o.OutputPath, "--base", o.BaseDir}, o.Members...),
	}
	return command.String()
}

func (o PackOperation) Run(_ context.Context, _ Runner) error {
	return archive.CreateTarGz(o.BaseDir, o.OutputPath, o.Members)
}

type UnpackOperation struct {
	ArchivePath string
	Destination string
}

func (o UnpackOperation) Display() string {
	command := Command{
		Name: "sendrecv",
		Args: []string{"unpack", "--archive", o.ArchivePath, "--dest", o.Destination},
	}
	return command.String()
}

func (o UnpackOperation) Run(_ context.Context, _ Runner) error {
	return archive.ExtractTarGz(o.ArchivePath, o.Destination)
}

type RemoveOperation struct {
	Path string
}

func (o RemoveOperation) Display() string {
	return Command{Name: "rm", Args: []string{"-f", o.Path}}.String()
}

func (o RemoveOperation) Run(_ context.Context, _ Runner) error {
	err := os.Remove(o.Path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

type MkdirOperation struct {
	Path string
}

func (o MkdirOperation) Display() string {
	return Command{Name: "mkdir", Args: []string{"-p", o.Path}}.String()
}

func (o MkdirOperation) Run(_ context.Context, _ Runner) error {
	return os.MkdirAll(o.Path, 0o755)
}

type MessageOperation struct {
	Message string
}

func (o MessageOperation) Display() string {
	return o.Message
}

func (o MessageOperation) Run(_ context.Context, runner Runner) error {
	fmt.Fprintln(runner.Exec.Stdout, o.Message)
	return nil
}

type Runner struct {
	Config      *config.Config
	Exec        ExecOptions
	RemoteProbe func(context.Context, *config.Config, *config.ResolvedHost) (doctor.RemoteCapabilities, error)
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
	sendMode := r.Config.Defaults.SendTransferMode
	if opts.SendMode != nil {
		sendMode = *opts.SendMode
	}
	decision, err := archive.Decide(paths)
	if err != nil {
		return nil, err
	}
	switch sendMode {
	case config.SendTransferModeRaw:
		decision = archive.Decision{Mode: archive.ModeRaw, Compressed: false, Reason: "send transfer mode forces raw rsync transfer"}
	case config.SendTransferModeArchive:
		decision = archive.Decision{Mode: archive.ModeArchive, Compressed: true, Reason: "send transfer mode forces archive transfer"}
	}
	extract := host.Extract
	if opts.Extract != nil {
		extract = *opts.Extract
	}
	if decision.Mode == archive.ModeRaw {
		plan := r.rawSendPlan(host, mappings)
		plan.Summary = decision.Reason
		if opts.Extract != nil {
			plan.Operations = append([]Operation{
				MessageOperation{Message: "note: extraction flags are ignored in raw send mode"},
			}, plan.Operations...)
		}
		return plan, nil
	}

	members := make([]string, 0, len(mappings))
	for _, mapping := range mappings {
		members = append(members, filepath.ToSlash(mapping.Target))
	}
	localArchive := filepath.Join(os.TempDir(), remote.ArchiveFileName)
	remoteArchive := sendArchivePath(host, extract, opts.KeepArchive)
	probe := r.remoteProbe()
	var capabilities doctor.RemoteCapabilities
	sendrecvPath := host.SendrecvPath
	if extract && !r.Exec.DryRun {
		capabilities, err = probe(context.Background(), r.Config, host)
		if err != nil {
			return nil, fmt.Errorf("remote capability probe failed before archive send: %w", err)
		}
		if !capabilities.RsyncOK {
			return nil, fmt.Errorf("remote rsync is unavailable on %s", host.SSHTarget)
		}
		if !capabilities.RemoteDirOK {
			return nil, fmt.Errorf("remote_dir %s is not accessible on %s", host.RemoteDir, host.SSHTarget)
		}
		if !capabilities.RemoteTempDirOK {
			return nil, fmt.Errorf("remote_temp_dir %s is not accessible on %s", host.RemoteTempDir, host.SSHTarget)
		}
		if capabilities.SendrecvPath != "" {
			sendrecvPath = capabilities.SendrecvPath
		}
		if !capabilities.SendrecvOK && !(capabilities.TarOK && capabilities.GzipOK) {
			remoteArchive = path.Join(host.RemoteDir, remote.ArchiveFileName)
		}
	}
	operations := []Operation{
		PackOperation{BaseDir: base, OutputPath: localArchive, Members: members},
		CommandOperation{Command: sshCommand(r.Config, host, remote.MkdirCommand(path.Dir(remoteArchive)))},
		CommandOperation{Command: rsyncCommand(r.Config, host, localArchive, host.SSHTarget+":"+remoteArchive)},
	}

	if extract {
		if r.Exec.DryRun {
			operations = append(operations,
				MessageOperation{Message: fmt.Sprintf("# runtime note: sendrecv will try remote %q first, then remote tar+gzip, and finally keep the archive at %s if no extractor is available", host.SendrecvPath, path.Join(host.RemoteDir, remote.ArchiveFileName))},
			)
		} else {
			if capabilities.SendrecvOK {
				operations = append(operations, CommandOperation{
					Command: sshCommand(r.Config, host, remote.UnpackCommand(sendrecvPath, remoteArchive, host.RemoteDir, opts.KeepArchive)),
				})
			} else if capabilities.TarOK && capabilities.GzipOK {
				operations = append(operations,
					MessageOperation{Message: fmt.Sprintf("warning: remote sendrecv %q not found on %s; using remote tar+gzip extraction fallback", host.SendrecvPath, host.SSHTarget)},
					CommandOperation{Command: sshCommand(r.Config, host, remote.TarExtractCommand(remoteArchive, host.RemoteDir, opts.KeepArchive))},
				)
			} else {
				operations = append(operations,
					MessageOperation{Message: fmt.Sprintf("warning: remote sendrecv %q and tar+gzip extraction are unavailable on %s; extraction skipped", host.SendrecvPath, host.SSHTarget)},
					MessageOperation{Message: fmt.Sprintf("result: archive uploaded to %s", remoteArchive)},
				)
			}
		}
	}
	operations = append(operations, RemoveOperation{Path: localArchive})
	return &Plan{
		Summary:    decision.Reason,
		Operations: operations,
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
	if len(paths) == 1 && archive.IsLikelyIncompressible(paths[0]) {
		return r.rawRecvPlan(host, paths[0], opts.PreserveTree)
	}

	cwd, err := filepath.Abs(".")
	if err != nil {
		return nil, err
	}
	remotePaths := resolveRemotePaths(host, paths)
	base := remoteBase(host, paths, remotePaths, opts.PreserveTree)
	members := relativeRemoteMembers(base, remotePaths)
	remoteArchive := remote.ArchivePath(host.RemoteTempDir)
	localArchive := recvArchivePath(cwd, extract, opts.KeepArchive)
	sendrecvPath := host.SendrecvPath
	if !r.Exec.DryRun {
		capabilities, err := r.remoteProbe()(context.Background(), r.Config, host)
		if err != nil {
			return nil, fmt.Errorf("remote capability probe failed before archive recv: %w", err)
		}
		if !capabilities.RsyncOK {
			return nil, fmt.Errorf("remote rsync is unavailable on %s", host.SSHTarget)
		}
		if !capabilities.SendrecvOK {
			return nil, fmt.Errorf("remote sendrecv is unavailable on %s", host.SSHTarget)
		}
		if !capabilities.RemoteTempDirOK {
			return nil, fmt.Errorf("remote_temp_dir %s is not accessible on %s", host.RemoteTempDir, host.SSHTarget)
		}
		if capabilities.SendrecvPath != "" {
			sendrecvPath = capabilities.SendrecvPath
		}
	}
	operations := []Operation{
		CommandOperation{Command: sshCommand(r.Config, host, remote.PackCommand(sendrecvPath, remoteArchive, base, members))},
		CommandOperation{Command: rsyncCommand(r.Config, host, host.SSHTarget+":"+remoteArchive, localArchive)},
		CommandOperation{Command: sshCommand(r.Config, host, remote.CleanupCommand(remoteArchive))},
	}
	if extract {
		operations = append(operations, UnpackOperation{ArchivePath: localArchive, Destination: cwd})
		if !opts.KeepArchive {
			operations = append(operations, RemoveOperation{Path: localArchive})
		}
	}
	return &Plan{
		Summary:    "receive remote files through archive pipeline",
		Operations: operations,
	}, nil
}

func (r Runner) rawSendPlan(host *config.ResolvedHost, mappings []pathmode.Mapping) *Plan {
	operations := make([]Operation, 0, len(mappings)*2)
	for _, mapping := range mappings {
		targetDir := path.Join(host.RemoteDir, path.Dir(filepath.ToSlash(mapping.Target)))
		operations = append(operations,
			CommandOperation{Command: sshCommand(r.Config, host, remote.MkdirCommand(targetDir))},
			CommandOperation{Command: rsyncCommand(r.Config, host, mapping.Source, host.SSHTarget+":"+targetDir+"/")},
		)
	}
	return &Plan{
		Summary:    "single incompressible file can transfer raw",
		Operations: operations,
	}
}

func (r Runner) rawRecvPlan(host *config.ResolvedHost, remoteInput string, preserveTree bool) (*Plan, error) {
	cwd, err := filepath.Abs(".")
	if err != nil {
		return nil, err
	}
	remotePath := resolveRemotePath(host, remoteInput)
	localRelative := recvRelativePath(host, remoteInput, remotePath, preserveTree)
	localTargetDir := filepath.Join(cwd, filepath.Dir(filepath.FromSlash(localRelative)))
	operations := []Operation{
		MkdirOperation{Path: localTargetDir},
		CommandOperation{Command: rsyncCommand(r.Config, host, host.SSHTarget+":"+remotePath, localTargetDir+string(filepath.Separator))},
	}
	return &Plan{
		Summary:    "single incompressible file can transfer raw",
		Operations: operations,
	}, nil
}

func (r Runner) Execute(ctx context.Context, plan *Plan) error {
	for _, operation := range plan.Operations {
		if r.Exec.DryRun {
			fmt.Fprintln(r.Exec.Stdout, operation.Display())
			continue
		}
		if r.Exec.Verbose {
			fmt.Fprintln(r.Exec.Stdout, operation.Display())
		}
		if err := operation.Run(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

func commonRemoteBase(paths []string) string {
	if len(paths) == 1 {
		return path.Dir(paths[0])
	}
	parts := strings.Split(strings.TrimPrefix(path.Clean(paths[0]), "/"), "/")
	for _, currentPath := range paths[1:] {
		current := strings.Split(strings.TrimPrefix(path.Clean(currentPath), "/"), "/")
		max := min(len(parts), len(current))
		var index int
		for index = 0; index < max && parts[index] == current[index]; index++ {
		}
		parts = parts[:index]
	}
	if len(parts) == 0 {
		return "/"
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

func rsyncCommand(cfg *config.Config, host *config.ResolvedHost, source, destination string) Command {
	return Command{
		Name: cfg.Tools.RSync,
		Args: rsyncArgs(cfg, host, source, destination),
	}
}

func rsyncArgs(cfg *config.Config, host *config.ResolvedHost, source, destination string) []string {
	args := append([]string{}, host.RsyncArgs...)
	if host.RemoteRsyncPath != "" {
		args = append(args, "--rsync-path="+host.RemoteRsyncPath)
	}
	args = append(args, "-e", strings.TrimSpace(strings.Join(append([]string{cfg.Tools.SSH}, host.SSHArgs...), " ")))
	args = append(args, source, destination)
	return args
}

func sshCommand(cfg *config.Config, host *config.ResolvedHost, remoteCommand string) Command {
	args := append([]string{}, host.SSHArgs...)
	args = append(args, host.SSHTarget, remoteCommand)
	return Command{Name: cfg.Tools.SSH, Args: args}
}

func sendArchivePath(host *config.ResolvedHost, extract, keepArchive bool) string {
	if !extract || keepArchive {
		return path.Join(host.RemoteDir, remote.ArchiveFileName)
	}
	return remote.ArchivePath(host.RemoteTempDir)
}

func (r Runner) remoteProbe() func(context.Context, *config.Config, *config.ResolvedHost) (doctor.RemoteCapabilities, error) {
	if r.RemoteProbe != nil {
		return r.RemoteProbe
	}
	return doctor.ProbeRemoteCapabilities
}

func recvArchivePath(cwd string, extract, keepArchive bool) string {
	if !extract || keepArchive {
		return filepath.Join(cwd, remote.ArchiveFileName)
	}
	return filepath.Join(os.TempDir(), remote.ArchiveFileName)
}

func resolveRemotePaths(host *config.ResolvedHost, paths []string) []string {
	resolved := make([]string, 0, len(paths))
	for _, current := range paths {
		resolved = append(resolved, resolveRemotePath(host, current))
	}
	return resolved
}

func resolveRemotePath(host *config.ResolvedHost, value string) string {
	if path.IsAbs(value) {
		return path.Clean(value)
	}
	return path.Clean(path.Join(host.RemoteDir, value))
}

func remoteBase(host *config.ResolvedHost, rawPaths, resolved []string, preserveTree bool) string {
	if !preserveTree {
		return commonRemoteBase(resolved)
	}
	allRelative := true
	for _, current := range rawPaths {
		if path.IsAbs(current) {
			allRelative = false
			break
		}
	}
	if allRelative {
		return path.Clean(host.RemoteDir)
	}
	return "/"
}

func relativeRemoteMembers(base string, resolved []string) []string {
	members := make([]string, 0, len(resolved))
	for _, current := range resolved {
		member := strings.TrimPrefix(strings.TrimPrefix(current, base), "/")
		if member == "" {
			member = path.Base(current)
		}
		members = append(members, member)
	}
	return members
}

func recvRelativePath(_ *config.ResolvedHost, input, resolved string, preserveTree bool) string {
	if !preserveTree {
		return path.Base(resolved)
	}
	if path.IsAbs(input) {
		return strings.TrimPrefix(path.Clean(input), "/")
	}
	return path.Clean(input)
}
