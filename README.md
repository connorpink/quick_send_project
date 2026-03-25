# sendrecv

[![GitHub Release](https://img.shields.io/github/v/release/connorpink/quick_send_project?sort=semver)](https://github.com/connorpink/quick_send_project/releases)
[![Homebrew Tap](https://img.shields.io/badge/homebrew-tap-success?logo=homebrew)](https://github.com/connorpink/homebrew-tap)
[![Release Downloads](https://img.shields.io/github/downloads/connorpink/quick_send_project/latest/total?label=release%20downloads)](https://github.com/connorpink/quick_send_project/releases/latest)

`sendrecv` is a Go CLI for repeat SSH-based file transfer between known devices. It keeps the transfer workflow in one binary while relying on `ssh` and `rsync` only at runtime.

## Status

v1 currently targets macOS and Linux. Windows support, a built-in TUI, and a native Yazi Lua plugin are out of scope for this release.

## Features

- `send` and `recv` subcommands built around host presets
- interactive host picking in `send`, with `fzf` first and Go fallback
- TOML config with per-host defaults
- automatic raw vs archive decision logic
- Go-native `tar.gz` archive packing and unpacking
- optional auto-extract on the destination side
- strip-common-prefix path mode by default
- opt-in `--preserve-tree`
- `--dry-run` and `--verbose`
- `doctor` checks for required tooling
- documented Yazi shell integration, including interactive host picking

## Install

See [docs/install.md](./docs/install.md).

## Quick start

```bash
sendrecv config init
$EDITOR ~/.config/sendrecv/config.toml
sendrecv config validate
sendrecv hosts
sendrecv send file.mp4
sendrecv send ./dir
sendrecv send --remote-host laptop ./dir
sendrecv recv laptop nested/file.txt
```

## Runtime dependencies

The local machine needs:

- `ssh`
- `rsync`

The local machine needs `ssh` and `rsync`, and the remote machine also needs `rsync` because transfers run through remote `rsync` over SSH.

For archive-mode `recv`, the remote machine must also have a compatible `sendrecv` binary available on `PATH`, in a standard Homebrew location, or at the configured `sendrecv_path`.

For archive-mode `send`, remote `sendrecv` is optional:

- if remote `sendrecv` exists and extraction is enabled, the archive is unpacked remotely
- if remote `sendrecv` is missing but remote `tar` and `gzip` exist, `sendrecv` falls back to shell extraction on the remote host
- if neither extraction path is available, `sendrecv` uploads the archive directly into `remote_dir` and prints the final archive path
- raw single-file transfers for incompressible files still work with just `ssh` and `rsync`

## Helper commands

Archive-mode remote execution uses:

- `sendrecv pack --output <archive> --base <dir> <members...>`
- `sendrecv unpack --archive <archive> --dest <dir>`

These are normal CLI commands and can be called over SSH by another `sendrecv` instance.

## Remote Doctor

`sendrecv doctor remote <host>` checks:

- remote `rsync`
- remote `sendrecv`
- remote `tar`
- remote `gzip`
- `remote_dir` readiness
- `remote_temp_dir` readiness

That makes it possible to see whether the host can do raw transfers only or full archive send/recv flows.

## Migration note

This version removes the old `tar`/`xz` runtime model. Existing configs using `tools.tar`, `tools.xz`, or `compression = "xz"` must be updated.

## Config

See [docs/config.md](./docs/config.md) and [examples/config.toml](./examples/config.toml).

## Yazi

Yazi is optional. The CLI remains the source of truth and Yazi should call `sendrecv`, not reimplement it. The recommended `g`, `s` integration is plain `sendrecv send`, which will pick a host interactively when needed. See [docs/yazi.md](./docs/yazi.md).

## Architecture

The package boundaries and transfer flow are documented in [ARCHITECTURE.md](.//ARCHITECTURE.md).
