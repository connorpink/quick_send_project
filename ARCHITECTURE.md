# Architecture

## Overview

`sendrecv` is a thin orchestrator around `ssh` and `rsync`. The Go code handles config loading, path mapping, archive decisions, command construction, Go-native `tar.gz` packing and unpacking, dry-run output, and diagnostics.

## Package boundaries

- `internal/cli`: Cobra command tree and flag handling
- `internal/config`: TOML parsing, validation, defaults, and host resolution
- `internal/archive`: raw vs archive decision logic plus Go-native `tar.gz` pack/unpack
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
5. In archive mode, build a local `.tar.gz` with Go, transfer it with `rsync`, and optionally extract remotely by running `sendrecv unpack` over `ssh`.

### recv

1. Resolve the host preset and requested remote paths.
2. Build a remote `.tar.gz` under the host temp directory by running `sendrecv pack` remotely.
3. Pull it back with `rsync`.
4. Optionally extract locally into the current working directory with the same Go-native unpacker.
5. Remove the remote staging archive after transfer; optionally keep the local archive when `--keep-archive` is set.

## Path handling

Default mode strips the common prefix across all inputs so that transfers recreate only the relative subtree under the destination root. `--preserve-tree` instead uses the current working directory as the base and keeps the invoked path tree intact.

## Shell assumptions

Remote commands use absolute destination paths and avoid interactive shell setup. Archive-mode remote execution assumes a compatible `sendrecv` binary is installed on the remote system and callable through `sendrecv_path`.
