# sendrecv

`sendrecv` is a Go CLI for repeat SSH-based file transfer between known devices. It keeps the transfer workflow in one binary while staying explicit about runtime tools: `ssh`, `rsync`, `tar`, and `xz`.

## Status

v1 currently targets macOS and Linux. Windows support, a built-in TUI, and a native Yazi Lua plugin are out of scope for this release.

## Features

- `send` and `recv` subcommands built around host presets
- TOML config with per-host defaults
- automatic raw vs archive decision logic
- `xz` compression for archive transfers
- optional auto-extract on the destination side
- strip-common-prefix path mode by default
- opt-in `--preserve-tree`
- `--dry-run` and `--verbose`
- `doctor` checks for required tooling
- documented Yazi shell integration

## Install

See [docs/install.md](/Users/connorpink/Code/quick_send_project/docs/install.md).

## Quick start

```bash
sendrecv config init
$EDITOR ~/.config/sendrecv/config.toml
sendrecv config validate
sendrecv hosts
sendrecv send laptop file.mp4
sendrecv send laptop ./dir
sendrecv recv laptop nested/file.txt
```

## Runtime dependencies

The local machine needs:

- `ssh`
- `rsync`
- `tar`
- `xz`

Remote archive extraction also expects `tar` and `xz` on the target host.

## Config

See [docs/config.md](/Users/connorpink/Code/quick_send_project/docs/config.md) and [examples/config.toml](/Users/connorpink/Code/quick_send_project/examples/config.toml).

## Yazi

Yazi is optional. The CLI remains the source of truth and Yazi should call `sendrecv`, not reimplement it. See [docs/yazi.md](/Users/connorpink/Code/quick_send_project/docs/yazi.md).

## Architecture

The package boundaries and transfer flow are documented in [ARCHITECTURE.md](/Users/connorpink/Code/quick_send_project/ARCHITECTURE.md).
