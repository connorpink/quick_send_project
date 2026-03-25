# Config

The default config path is:

- macOS: `~/Library/Application Support/sendrecv/config.toml`
- Linux: `~/.config/sendrecv/config.toml`

You can override it with `--config` or `SENDRECV_CONFIG`.

## Example

```toml
[defaults]
extract = true
compression = "gzip"
remote_temp_dir = "/tmp/sendrecv"
rsync_args = ["--archive", "--partial"]
ssh_args = ["-o", "BatchMode=yes"]

[tools]
ssh = "ssh"
rsync = "rsync"

[hosts.laptop]
ssh_target = "user@laptop"
sendrecv_path = "sendrecv"
remote_dir = "/home/user/Incoming"

[hosts.server]
ssh_target = "deploy@example"
sendrecv_path = "/usr/local/bin/sendrecv"
remote_dir = "/srv/incoming"
extract = true
remote_temp_dir = "/tmp/sendrecv"
rsync_args = ["--archive", "--partial", "--info=progress2"]
```

## Fields

- `defaults.extract`: default extraction behavior for send/recv archive flows
- `defaults.compression`: must be `"gzip"` in this release
- `defaults.remote_temp_dir`: absolute remote staging directory
- `defaults.rsync_args`: appended before transfer source and destination
- `defaults.ssh_args`: prepended to every `ssh` call
- `tools.*`: executable names or absolute paths for required tools
- `hosts.<name>.ssh_target`: SSH target such as `user@host`
- `hosts.<name>.sendrecv_path`: optional remote binary path, defaults to `sendrecv`
- `hosts.<name>.remote_dir`: absolute default destination directory on the remote host
- `hosts.<name>.remote_temp_dir`: optional per-host override for archive staging
- `hosts.<name>.extract`: optional per-host override
- `hosts.<name>.rsync_args`: extra host-specific rsync args
- `hosts.<name>.ssh_args`: extra host-specific ssh args

## Validation rules

- At least one host must exist.
- `remote_dir` must be absolute.
- `remote_temp_dir` must be absolute when set.
- Tool values must be a bare executable name or an absolute path.
- `sendrecv_path` must be a bare executable name or an absolute path when set.
- Compression is fixed to `gzip` for this release.
- Unknown config keys are rejected, including the removed `tools.tar` and `tools.xz` fields.

## Remote binary requirement

Remote `rsync` is required for all transfers.

Archive-mode `recv` requires `sendrecv` on the remote host because remote archive creation is executed through `sendrecv pack`.

Archive-mode `send` prefers `sendrecv` on the remote host for extraction through `sendrecv unpack`.

When `sendrecv_path` is left as the default `sendrecv`, remote detection first checks `command -v sendrecv` and then falls back to common Homebrew install paths:

- `/home/linuxbrew/.linuxbrew/bin/sendrecv`
- `/opt/homebrew/bin/sendrecv`
- `/usr/local/bin/sendrecv`

Set `sendrecv_path` explicitly only if the remote binary lives somewhere else.

If remote `sendrecv` is missing and extraction was requested, `sendrecv` falls back in this order:

1. remote `tar` + `gzip` extraction
2. archive upload directly into `remote_dir` with a warning and printed archive location
