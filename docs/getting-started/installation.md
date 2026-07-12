---
title: Installation
parent: Getting started
nav_order: 2
---

# Installing Trove

Trove ships as a single Go binary (and Docker image). With a valid config file,
`trove` opens the journal, discovers source modules, and supervises them. New here?
Follow [Try in a day](./try-in-a-day.md) for a guided first run.

## Release channels

| Channel | Source | Use when |
|---------|--------|----------|
| **Stable** | Semver tags (`v0.1.0`) on [GitHub Releases](https://github.com/joshmcarthur/trove/releases) | Production installs, package managers |
| **Rolling** | `latest` prerelease (rebuilt on every `main` push) | Trying the newest changes |

Stable releases ship raw precompiled binaries, `.tar.gz`/`.zip` archives,
`.deb`/`.rpm` packages, and `checksums.txt`.
The rolling `latest` channel ships raw platform binaries.

## Homebrew (macOS and Linux)

After the `homebrew-trove` tap is configured:

```bash
brew install joshmcarthur/trove/trove
trove -version
```

## Debian and Ubuntu

Download the `.deb` for your architecture from the stable release page, then:

```bash
curl -LO https://github.com/joshmcarthur/trove/releases/download/v0.1.0/trove_0.1.0_linux_amd64.deb
sudo dpkg -i trove_0.1.0_linux_amd64.deb
trove -version
```

Replace `v0.1.0` and `amd64` with the version and architecture you need.

## Fedora and RHEL

```bash
curl -LO https://github.com/joshmcarthur/trove/releases/download/v0.1.0/trove_0.1.0_linux_amd64.rpm
sudo rpm -i trove_0.1.0_linux_amd64.rpm
trove -version
```

## From releases (manual download)

### Stable (recommended)

```bash
VERSION=v0.1.0

# Linux amd64 — raw binary (no extract step)
curl -LO "https://github.com/joshmcarthur/trove/releases/download/${VERSION}/trove-linux-amd64"
curl -LO "https://github.com/joshmcarthur/trove/releases/download/${VERSION}/checksums.txt"
sha256sum -c checksums.txt --ignore-missing
chmod +x trove-linux-amd64
sudo install -m 755 trove-linux-amd64 /usr/local/bin/trove

# macOS arm64 — raw binary
curl -LO "https://github.com/joshmcarthur/trove/releases/download/${VERSION}/trove-darwin-arm64"
curl -LO "https://github.com/joshmcarthur/trove/releases/download/${VERSION}/checksums.txt"
shasum -a 256 -c checksums.txt --ignore-missing
xattr -d com.apple.quarantine trove-darwin-arm64
chmod +x trove-darwin-arm64
sudo install -m 755 trove-darwin-arm64 /usr/local/bin/trove
```

Archives are also available if you prefer a tarball (includes `LICENSE`):

```bash
curl -LO "https://github.com/joshmcarthur/trove/releases/download/${VERSION}/trove_0.1.0_linux_amd64.tar.gz"
tar -xzf trove_0.1.0_linux_amd64.tar.gz
sudo install -m 755 trove /usr/local/bin/trove
```

### Rolling (`latest`)

```bash
# Linux amd64
curl -LO https://github.com/joshmcarthur/trove/releases/latest/download/trove-linux-amd64
curl -LO https://github.com/joshmcarthur/trove/releases/latest/download/checksums.txt
sha256sum -c checksums.txt --ignore-missing
chmod +x trove-linux-amd64

# macOS arm64
curl -LO https://github.com/joshmcarthur/trove/releases/latest/download/trove-darwin-arm64
curl -LO https://github.com/joshmcarthur/trove/releases/latest/download/checksums.txt
shasum -a 256 -c checksums.txt --ignore-missing
xattr -d com.apple.quarantine trove-darwin-arm64
chmod +x trove-darwin-arm64
```

Windows: download `trove-windows-amd64.exe` from the release page.

### macOS Gatekeeper

Release binaries are not Apple-signed yet. After downloading, macOS tags the file
with a quarantine flag and Gatekeeper may block it with a message like *"Apple
could not verify … is free of malware"*.

Remove the quarantine flag before running or installing:

```bash
xattr -d com.apple.quarantine trove-darwin-arm64
```

If you already installed to `/usr/local/bin/trove`, clear it there too:

```bash
xattr -d com.apple.quarantine /usr/local/bin/trove
```

`xattr` exits with an error when the attribute is missing — that is fine.

Alternatives if Gatekeeper still blocks:

- **System Settings → Privacy & Security** — after a blocked run, an **Open Anyway**
  button may appear for Trove.
- **Build from source** on your Mac (see below) — locally built binaries are not
  quarantined.

Verify:

```bash
trove -version
# 0.1.0 (abc1234, 2026-07-12)
```

## Build from source

Requires Go 1.26+ ([mise](https://mise.jdx.dev/) recommended — see `.mise.toml`).

```bash
git clone https://github.com/joshmcarthur/trove.git
cd trove
make build
./bin/trove -version
```

`make build` injects version metadata from `git describe` and the current commit.

## Docker

```bash
docker pull ghcr.io/joshmcarthur/trove:latest
docker run --rm ghcr.io/joshmcarthur/trove:latest -version
```

Pin a stable release with a semver tag, e.g. `ghcr.io/joshmcarthur/trove:0.1.0`.

## External modules

Release binaries include built-in modules (`http-ingest`, `mcp-query`,
`type-catalog`). Optional modules (`mqtt-source`, `telegram-source`,
`http-gateway`, etc.) are not bundled — build them with `make build` and install
under `[modules].paths`. See [Building modules](../building-modules.md).

## System requirements

- **Binary:** none beyond the platform itself (static Go build)
- **Docker:** any OCI runtime
- **Development:** Go 1.26+, `golangci-lint` for `make lint`; Deno 2.9+ only under
  `./docs` for the documentation site
