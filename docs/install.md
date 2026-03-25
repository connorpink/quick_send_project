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

Download the latest archive for your OS and architecture directly from the latest GitHub release:

```bash
curl -LO https://github.com/connorpink/quick_send_project/releases/latest/download/sendrecv_Darwin_arm64.tar.gz
tar -xzf sendrecv_Darwin_arm64.tar.gz
chmod +x sendrecv
./sendrecv --help
```

Example archive names:

- macOS Apple Silicon: `sendrecv_Darwin_arm64.tar.gz`
- macOS Intel: `sendrecv_Darwin_x86_64.tar.gz`
- Linux x86_64: `sendrecv_Linux_x86_64.tar.gz`
- Linux arm64: `sendrecv_Linux_arm64.tar.gz`

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

Remote archive extraction also expects sendrecv binary on the target host. If binary not found, will fall back to tar and xz for unpacking `tar` and `xz` on the target host. If no method of unpacking is found, it will send the archive as is.

If any runtime dependency is missing after installation, run:

```bash
sendrecv doctor
```
