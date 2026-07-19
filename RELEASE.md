# Release Process

## Creating a Release

1. Ensure all changes are committed and pushed to main
2. Create and push a version tag:

```bash
git tag v1.0.0
git push origin v1.0.0
```

## Go Install

After pushing the tag, users can install with:

```bash
go install github.com/williamokano/claude-status-line-go@latest
```

Or a specific version:

```bash
go install github.com/williamokano/claude-status-line-go@v1.0.0
```

## Versioning

This project uses [Semantic Versioning](https://semver.org/).

- `v1.0.0` - Initial stable release
- `v1.x.x` - Backwards-compatible features/fixes
- `v2.x.x` - Breaking changes

## GitHub Actions (Optional)

For automated releases, add `.github/release.yml`:

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  release:
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - uses: goreleaser/goreleaser-action@v6
        with:
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

With goreleaser, add `.goreleaser.yml` for cross-platform builds.