---
title: Installation
parent: Getting started
nav_order: 2
---

# Installing Trove

Trove ships as a single Go binary (and Docker image). With a valid config file,
`trove` opens the journal, discovers source modules, and supervises them. New here?
Follow [Try in a day](./try-in-a-day.md) for a guided first run.

## From releases

Download the latest build for your platform from
[GitHub Releases](https://github.com/joshmcarthur/trove/releases).

```bash
# Linux amd64
curl -LO https://github.com/joshmcarthur/trove/releases/latest/download/trove-linux-amd64
chmod +x trove-linux-amd64

# macOS arm64
curl -LO https://github.com/joshmcarthur/trove/releases/latest/download/trove-darwin-arm64
chmod +x trove-darwin-arm64

# Windows amd64
# Download trove-windows-amd64.exe from the release page
```

Verify:

```bash
./trove-darwin-arm64 -version
```

## Build from source

Requires Go 1.26+ ([mise](https://mise.jdx.dev/) recommended — see `.mise.toml`).

```bash
git clone https://github.com/joshmcarthur/trove.git
cd trove
make build
./bin/trove -version
```

## Docker

```bash
docker pull ghcr.io/joshmcarthur/trove:latest
docker run --rm ghcr.io/joshmcarthur/trove:latest -version
```

## System requirements

- **Binary:** none beyond the platform itself (static Go build)
- **Docker:** any OCI runtime
- **Development:** Go 1.26+, `golangci-lint` for `make lint`; Deno 2.9+ only under
  `./docs` for the documentation site
