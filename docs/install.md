# Install

## Build from source

```bash
git clone <repo>
cd sendrecv
go build ./cmd/sendrecv
```

## Binary release

The release workflow is configured for macOS and Linux archives via GoReleaser.

## Homebrew

The repository includes GoReleaser configuration for Homebrew tap generation. Fill in the tap owner and repository names before the first public release.
