# Architecture

## Overview

`sendrecv` is a thin orchestrator around explicit external tools. The Go code handles config loading, path mapping, archive decisions, command construction, dry-run output, and diagnostics. File transfer and compression remain delegated to `ssh`, `rsync`, `tar`, and `xz`.

## Package boundaries

- `internal/cli`: Cobra command tree and flag handling
- `internal/config`: TOML parsing, validation, defaults, and host resolution
- `internal/archive`: raw vs archive/compression decision logic
- `internal/pathmode`: common-prefix stripping and preserve-tree mapping
- `internal/transport`: transfer planning and command execution
- `internal/remote`: remote shell command helpers
- `internal/doctor`: local and remote dependency checks
- `internal/yazi`: example Yazi integration snippets

## Transfer model

### send

1. Resolve the host preset and merged defaults.
2. Build path mappings.
3. Decide raw vs archive mode.
4. In raw mode, `rsync` the selected file directly to the destination directory.
5. In archive mode, create a `tar` stream rooted at the shared base path, compress with `xz`, transfer the archive with `rsync`, and optionally extract remotely with a non-interactive `ssh` command.

### recv

1. Resolve the host preset and requested remote paths.
2. Build a remote archive under the host temp directory.
3. Pull it back with `rsync`.
4. Optionally extract locally into the current working directory.
5. Remove the remote archive unless `--keep-archive` is set.

## Path handling

Default mode strips the common prefix across all inputs so that transfers recreate only the relative subtree under the destination root. `--preserve-tree` instead uses the current working directory as the base and keeps the invoked path tree intact.

## Shell assumptions

Remote commands use absolute destination paths and avoid interactive shell setup. The CLI assumes only a POSIX shell, `tar`, and `xz` on the remote system.
