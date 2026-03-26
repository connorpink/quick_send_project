package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"sendrecv/internal/archive"
	"sendrecv/internal/config"
	"sendrecv/internal/doctor"
	"sendrecv/internal/transport"
	"sendrecv/internal/yazi"
)

type rootOptions struct {
	ConfigPath string
	Verbose    bool
	DryRun     bool
	GoFuzzy    bool
}

type transferFlags struct {
	Extract      bool
	ExtractSet   bool
	KeepArchive  bool
	PreserveTree bool
}

func NewRootCommand() *cobra.Command {
	opts := &rootOptions{}
	cmd := &cobra.Command{
		Use:   "sendrecv",
		Short: "Repeat SSH-based file transfer between known devices",
	}
	cmd.PersistentFlags().StringVar(&opts.ConfigPath, "config", "", "config file path")
	cmd.PersistentFlags().BoolVar(&opts.Verbose, "verbose", false, "print commands before running them")
	cmd.PersistentFlags().BoolVar(&opts.DryRun, "dry-run", false, "print commands without running them")
	cmd.PersistentFlags().BoolVar(&opts.GoFuzzy, "go-fuzzy", false, "force the Go fuzzy picker even if fzf is installed")

	cmd.AddCommand(newConfigCommand(opts))
	cmd.AddCommand(newHostsCommand(opts))
	cmd.AddCommand(newDoctorCommand(opts))
	cmd.AddCommand(newSendCommand(opts))
	cmd.AddCommand(newRecvCommand(opts))
	cmd.AddCommand(newPackCommand())
	cmd.AddCommand(newUnpackCommand())
	cmd.AddCommand(newYaziCommand())
	cmd.AddCommand(newYaziExampleCommand())
	return cmd
}

func newConfigCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Manage config"}
	cmd.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Write a config file and optionally import SSH hosts",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigInit(cmd.OutOrStdout(), cmd.ErrOrStderr(), opts)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "validate",
		Short: "Validate config",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadConfig(opts.ConfigPath)
			if err != nil {
				return err
			}
			if err := cfg.Validate(); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "config is valid")
			return nil
		},
	})
	return cmd
}

func newHostsCommand(opts *rootOptions) *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "hosts",
		Short: "List configured hosts",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadConfig(opts.ConfigPath)
			if err != nil {
				return err
			}
			names := make([]string, 0, len(cfg.Hosts))
			for name := range cfg.Hosts {
				names = append(names, name)
			}
			sort.Strings(names)
			if jsonOutput {
				payload := make([]hostSummary, 0, len(names))
				for _, name := range names {
					host, _ := cfg.ResolveHost(name)
					payload = append(payload, hostSummary{
						Name:      name,
						SSHTarget: host.SSHTarget,
						RemoteDir: host.RemoteDir,
					})
				}
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(payload)
			}
			for _, name := range names {
				host, _ := cfg.ResolveHost(name)
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", name, host.SSHTarget, host.RemoteDir)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "print hosts in JSON format")
	return cmd
}

type hostSummary struct {
	Name      string `json:"name"`
	SSHTarget string `json:"ssh_target"`
	RemoteDir string `json:"remote_dir"`
}

func newDoctorCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check local configuration and runtime dependencies",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolvedConfigPath(opts.ConfigPath)
			if err != nil {
				return err
			}
			for _, check := range doctor.LocalChecks(path) {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", check.Name, check.Status, check.Detail)
			}
			return nil
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "remote <host>",
		Short: "Check remote host capabilities",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadConfig(opts.ConfigPath)
			if err != nil {
				return err
			}
			host, err := cfg.ResolveHost(args[0])
			if err != nil {
				return err
			}
			for _, check := range doctor.RemoteChecks(cmd.Context(), cfg, host) {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", check.Name, check.Status, check.Detail)
			}
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "yazi",
		Short: "Check Yazi integration and print the recommended keymap snippet",
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, check := range doctor.YaziChecks() {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", check.Name, check.Status, check.Detail)
			}
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintln(cmd.OutOrStdout(), "Add the following snippet to your Yazi keymap file to enable interactive sendrecv host selection:")
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprint(cmd.OutOrStdout(), strings.TrimLeft(yazi.ExamplePickerKeymap(), "\n"))
			return nil
		},
	})
	return cmd
}

func newSendCommand(opts *rootOptions) *cobra.Command {
	flags := &transferFlags{}
	var remoteHost string
	cmd := &cobra.Command{
		Use:   "send <paths...>",
		Short: "Send local files to a configured host",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadConfig(opts.ConfigPath)
			if err != nil {
				return err
			}
			host, err := selectSendHost(cmd.Context(), cfg, remoteHost, opts, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			runner := transport.Runner{
				Config: cfg,
				Exec: transport.ExecOptions{
					DryRun:  opts.DryRun,
					Verbose: opts.Verbose,
					Stdout:  cmd.OutOrStdout(),
					Stderr:  cmd.ErrOrStderr(),
				},
			}
			plan, err := runner.SendPlan(host, args, transferOptions(flags))
			if err != nil {
				return err
			}
			if opts.Verbose || opts.DryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "# selected host: %s\n", host.Name)
				fmt.Fprintf(cmd.OutOrStdout(), "# %s\n", plan.Summary)
			}
			return runner.Execute(context.Background(), plan)
		},
	}
	cmd.Flags().StringVar(&remoteHost, "remote-host", "", "configured host name to use without interactive selection")
	attachTransferFlags(cmd, flags)
	return cmd
}

func newRecvCommand(opts *rootOptions) *cobra.Command {
	flags := &transferFlags{}
	cmd := &cobra.Command{
		Use:   "recv <host> <paths...>",
		Short: "Receive remote files from a configured host",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadConfig(opts.ConfigPath)
			if err != nil {
				return err
			}
			host, err := cfg.ResolveHost(args[0])
			if err != nil {
				return err
			}
			runner := transport.Runner{
				Config: cfg,
				Exec: transport.ExecOptions{
					DryRun:  opts.DryRun,
					Verbose: opts.Verbose,
					Stdout:  cmd.OutOrStdout(),
					Stderr:  cmd.ErrOrStderr(),
				},
			}
			plan, err := runner.RecvPlan(host, args[1:], transferOptions(flags))
			if err != nil {
				return err
			}
			if opts.Verbose || opts.DryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "# %s\n", plan.Summary)
			}
			return runner.Execute(context.Background(), plan)
		},
	}
	attachTransferFlags(cmd, flags)
	return cmd
}

func newYaziCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "yazi",
		Short: "Helpers for Yazi integration",
	}
	cmd.AddCommand(newYaziSnippetCommand())
	return cmd
}

func newYaziSnippetCommand() *cobra.Command {
	var picker bool
	cmd := &cobra.Command{
		Use:   "snippet [host]",
		Short: "Print an example Yazi keymap snippet",
		Args: func(cmd *cobra.Command, args []string) error {
			if picker {
				return cobra.NoArgs(cmd, args)
			}
			return cobra.ExactArgs(1)(cmd, args)
		},
		Run: func(cmd *cobra.Command, args []string) {
			if picker {
				fmt.Fprint(cmd.OutOrStdout(), strings.TrimLeft(yazi.ExamplePickerKeymap(), "\n"))
				return
			}
			fmt.Fprint(cmd.OutOrStdout(), strings.TrimLeft(yazi.ExampleKeymap(args[0]), "\n"))
		},
	}
	cmd.Flags().BoolVar(&picker, "picker", false, "print the interactive host-picker snippet")
	return cmd
}

func newYaziExampleCommand() *cobra.Command {
	var picker bool
	cmd := &cobra.Command{
		Use:    "yazi-example [host]",
		Short:  "Print an example Yazi keymap snippet",
		Hidden: true,
		Args: func(cmd *cobra.Command, args []string) error {
			if picker {
				return cobra.NoArgs(cmd, args)
			}
			return cobra.ExactArgs(1)(cmd, args)
		},
		Run: func(cmd *cobra.Command, args []string) {
			if picker {
				fmt.Fprint(cmd.OutOrStdout(), strings.TrimLeft(yazi.ExamplePickerKeymap(), "\n"))
				return
			}
			fmt.Fprint(cmd.OutOrStdout(), strings.TrimLeft(yazi.ExampleKeymap(args[0]), "\n"))
		},
	}
	cmd.Flags().BoolVar(&picker, "picker", false, "print the interactive host-picker snippet")
	return cmd
}

func newPackCommand() *cobra.Command {
	var output string
	var base string
	cmd := &cobra.Command{
		Use:   "pack --output <archive> --base <dir> <members...>",
		Short: "Create a gzip-compressed tar archive with Go-native packing",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if output == "" {
				return fmt.Errorf("--output is required")
			}
			if base == "" {
				return fmt.Errorf("--base is required")
			}
			baseDir, err := filepath.Abs(base)
			if err != nil {
				return err
			}
			return archive.CreateTarGz(baseDir, output, args)
		},
	}
	cmd.Flags().StringVar(&output, "output", "", "archive file path")
	cmd.Flags().StringVar(&base, "base", "", "base directory for archive members")
	return cmd
}

func newUnpackCommand() *cobra.Command {
	var archivePath string
	var destination string
	cmd := &cobra.Command{
		Use:   "unpack --archive <archive> --dest <dir>",
		Short: "Extract a gzip-compressed tar archive with Go-native unpacking",
		RunE: func(cmd *cobra.Command, args []string) error {
			if archivePath == "" {
				return fmt.Errorf("--archive is required")
			}
			if destination == "" {
				return fmt.Errorf("--dest is required")
			}
			destDir, err := filepath.Abs(destination)
			if err != nil {
				return err
			}
			return archive.ExtractTarGz(archivePath, destDir)
		},
	}
	cmd.Flags().StringVar(&archivePath, "archive", "", "archive file path")
	cmd.Flags().StringVar(&destination, "dest", "", "absolute destination directory")
	return cmd
}

func attachTransferFlags(cmd *cobra.Command, flags *transferFlags) {
	cmd.Flags().BoolVar(&flags.Extract, "extract", false, "force extraction on the destination side")
	cmd.Flags().BoolVar(&flags.KeepArchive, "keep-archive", false, "keep the transferred archive after extraction")
	cmd.Flags().BoolVar(&flags.PreserveTree, "preserve-tree", false, "preserve the provided path tree instead of stripping the common prefix")
	cmd.Flags().Bool("no-extract", false, "disable extraction on the destination side")
	_ = cmd.Flags().Lookup("extract").NoOptDefVal
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed("extract") && cmd.Flags().Changed("no-extract") {
			return fmt.Errorf("use only one of --extract or --no-extract")
		}
		if cmd.Flags().Changed("extract") {
			flags.Extract = true
			flags.ExtractSet = true
		}
		if cmd.Flags().Changed("no-extract") {
			flags.Extract = false
			flags.ExtractSet = true
		}
		return nil
	}
}

func transferOptions(flags *transferFlags) transport.TransferOptions {
	opts := transport.TransferOptions{
		KeepArchive:  flags.KeepArchive,
		PreserveTree: flags.PreserveTree,
	}
	if flags.ExtractSet {
		extract := flags.Extract
		opts.Extract = &extract
	}
	return opts
}

func loadConfig(override string) (*config.Config, string, error) {
	path, err := resolvedConfigPath(override)
	if err != nil {
		return nil, "", err
	}
	cfg, err := config.Load(path)
	if err != nil && os.IsNotExist(err) {
		return nil, path, fmt.Errorf("config not found at %s; run `sendrecv config init`", path)
	}
	return cfg, path, err
}

func resolvedConfigPath(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	if env := os.Getenv("SENDRECV_CONFIG"); env != "" {
		return env, nil
	}
	return config.DefaultPath()
}
