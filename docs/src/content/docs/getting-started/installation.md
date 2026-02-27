---
title: Installation
description: How to install society
---

## Quick install

Download the latest release binary (macOS / Linux):

```bash
curl -fsSL https://society.luischav.es/install.sh | sh
```

This detects your OS and architecture, downloads the binary, verifies the checksum, and installs to `/usr/local/bin` (or `~/.local/bin` if not writable).

## Update

If you already have society installed:

```bash
society update
```

This checks GitHub for the latest release and replaces the binary in place.

## From source

Society is a single Go binary with no runtime dependencies.

```bash
git clone https://github.com/lucharo/society.git
cd society
go build -o society ./cmd/society
```

Move the binary somewhere on your PATH:

```bash
mv society ~/.local/bin/
```

### Cross-compile for a remote machine

To build for a Linux server (e.g., a home server running on x86_64):

```bash
GOOS=linux GOARCH=amd64 go build -o society-linux ./cmd/society
scp society-linux user@server:/usr/local/bin/society
```

For ARM (e.g., Raspberry Pi):

```bash
GOOS=linux GOARCH=arm64 go build -o society-arm64 ./cmd/society
```

## Docker

```bash
docker build -t society .
docker run society list
```

The Docker image includes the example agent configs in `/etc/society/agents/`.

## Verify installation

```bash
society
```

You should see the usage help with all available commands.
