# Install

GitHub Releases are the source of truth for published binaries and Linux packages.

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

Download the archive for your OS and architecture from the GitHub Releases page:

```bash
curl -LO https://github.com/connorpink/quick_send_project/releases/download/v0.1.0/sendrecv_v0.1.0_Darwin_arm64.tar.gz
tar -xzf sendrecv_v0.1.0_Darwin_arm64.tar.gz
chmod +x sendrecv
./sendrecv --help
```

## Homebrew

Install from the project tap:

```bash
brew install connorpink/tap/sendrecv
```

## Debian / Ubuntu

Download the `.deb` package from the GitHub release and install it:

```bash
sudo dpkg -i ./sendrecv_0.1.0_linux_amd64.deb
sendrecv --help
```

## RPM-based distributions

Download the `.rpm` package from the GitHub release and install it:

```bash
sudo rpm -i ./sendrecv_0.1.0_linux_amd64.rpm
sendrecv --help
```

## Runtime prerequisites

The local machine needs:

- `ssh`
- `rsync`
- `tar`
- `xz`

Remote archive extraction also expects `tar` and `xz` on the target host.

If any runtime dependency is missing after installation, run:

```bash
sendrecv doctor
```
