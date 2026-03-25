# Install

## Build from source

```bash
git clone <repo>
cd sendrecv
go build ./cmd/sendrecv
```

The local machine needs `ssh` and `rsync`. The remote machine also needs `rsync`.

If you want archive-mode `recv` or remote extraction for archive-mode `send`, install `sendrecv` on the remote machine as well.

If remote `sendrecv` is missing, archive-mode `send` can still work:

- it will try remote `tar` + `gzip` extraction if available
- otherwise it will upload the archive directly into the destination directory and tell you where it landed

## Binary release

The release workflow is configured for macOS and Linux archives via GoReleaser.

## Homebrew

The repository includes GoReleaser configuration for Homebrew tap generation. Fill in the tap owner and repository names before the first public release.
