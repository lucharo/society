---
title: SSH Transports
description: Reach agents on remote machines through SSH
---

Society has two SSH-based transports:

- **SSH (tunnel)** — Opens an SSH tunnel to forward traffic to a remote A2A daemon
- **SSH Exec** — SSHes into a remote host and runs a CLI command directly (no daemon needed)

## SSH Tunnel Transport

Opens an SSH tunnel to the remote host and forwards a local port to the agent's port. All JSON-RPC traffic flows through the tunnel.

### When to use

- Remote servers running a Society daemon with agents
- Machines behind firewalls where ports aren't directly exposed

### Registration

```bash
society onboard --manual
```

```
Agent name: server-claude
Description: Claude on my server
Transport [http/ssh/ssh-exec/docker/stdio] (http): ssh
SSH host: my-server
SSH user: deploy
SSH key path: ~/.ssh/id_ed25519
SSH port (22): 22
Agent port on remote host (8080): 8003
Skills (comma-separated IDs, or empty): code
```

### Registry entry

```json
{
  "name": "server-claude",
  "url": "http://localhost:8003",
  "description": "Claude on my server",
  "transport": {
    "type": "ssh",
    "config": {
      "host": "my-server",
      "user": "deploy",
      "key_path": "/home/you/.ssh/id_ed25519",
      "port": "22",
      "forward_port": "8003"
    }
  }
}
```

### Config reference

| Key | Required | Default | Description |
|-----|----------|---------|-------------|
| `host` | Yes | — | Remote hostname or IP |
| `user` | Yes | — | SSH username |
| `key_path` | Yes | — | Path to SSH private key |
| `port` | No | `22` | SSH port |
| `forward_port` | No | `8080` | Agent port on the remote host |

### How it works

1. Reads and parses the SSH private key
2. Dials an SSH connection to `host:port`
3. Opens a local TCP listener on a random port
4. Forwards connections through the SSH tunnel to `127.0.0.1:forward_port` on the remote
5. Sends HTTP POST to the local forwarded port
6. Closes tunnel and SSH connection when done

Each `send` command opens a fresh tunnel. The tunnel is short-lived — it's created, used for one request, and torn down.

---

## SSH Exec Transport

SSHes into a remote host, runs a CLI command (e.g., `claude`, `codex`), and returns the output. No daemon or server needed on the remote machine — just the CLI tool installed.

### When to use

- Remote machines with CLI tools like `claude` or `codex` installed
- When you don't want to install or run a Society daemon on the remote
- Quick setup — deep scan auto-detects available CLIs

### Auto-detection

Deep scan finds CLI tools on SSH hosts automatically:

```bash
society onboard --deep
```

```
✓ Found 2 remote CLI tools via SSH: arch-claude, arch-codex
```

This SSHes into each host from your `~/.ssh/config`, runs `command -v` for known CLIs, and registers any it finds. The detected path is absolute, so it works even when the SSH session has a different PATH than interactive shells.

### Manual registration

```bash
society onboard --manual
```

```
Agent name: server-claude
Transport [http/ssh/ssh-exec/docker/stdio] (http): ssh-exec
SSH host: my-server
SSH user: deploy
SSH key path: ~/.ssh/id_ed25519
SSH port (22): 22
Remote command: claude
Command args (-p --output-format json):
```

### Registry entry

```json
{
  "name": "server-claude",
  "url": "ssh-exec://my-server/claude",
  "description": "Claude via SSH exec",
  "transport": {
    "type": "ssh-exec",
    "config": {
      "host": "my-server",
      "user": "deploy",
      "key_path": "/home/you/.ssh/id_ed25519",
      "port": "22",
      "command": "/usr/local/bin/claude",
      "args": "-p --output-format json"
    }
  }
}
```

### Config reference

| Key | Required | Default | Description |
|-----|----------|---------|-------------|
| `host` | Yes | — | Remote hostname or IP |
| `user` | Yes | — | SSH username |
| `key_path` | Yes | — | Path to SSH private key |
| `port` | No | `22` | SSH port |
| `command` | Yes | — | CLI command to run (use absolute path) |
| `args` | No | — | Default arguments for the command |

### How it works

1. Reads and parses the SSH private key
2. Dials an SSH connection to `host:port`
3. Creates an SSH session and runs: `command args 'user message'`
4. Captures stdout/stderr
5. Parses the output (JSON if available, plain text otherwise)
6. Returns as an A2A task response

The user message is shell-escaped (single-quoted with proper escaping) for security.

---

## Tailscale + SSH

A common pattern is using [Tailscale](https://tailscale.com) hostnames with either SSH transport:

```
SSH host: arch        # Tailscale hostname
SSH user: luis
SSH key path: ~/.ssh/id_ed25519
```

Tailscale handles the networking, SSH handles the connection. No need to expose ports publicly.

### SSH server requirement

Both SSH transports require an SSH server on the remote host. If your remote is a **macOS machine**, note that sshd is disabled by default. You have two options:

1. **macOS Remote Login** — System Settings > General > Sharing > Remote Login. Opens port 22 on all interfaces.

2. **Tailscale SSH** — Run `sudo tailscale set --ssh` on the remote. This is more secure as it only listens on the Tailscale interface and uses Tailscale identity for auth. However, this **does not work with the macOS App Store version** of Tailscale (sandboxed). You need the [standalone build](https://tailscale.com/download) instead.

## Troubleshooting

**"connection refused" on port 22:**
The remote machine doesn't have an SSH server running. On macOS, enable Remote Login or Tailscale SSH (see above). On Linux, ensure `sshd` is running (`systemctl status sshd`).

**"connection refused" after SSH connects (tunnel transport):**
The SSH tunnel is working but nothing is listening on the remote port. Make sure the agent is running on the remote machine:

```bash
ssh user@server "ss -tlnp | grep 8003"
```

**"command not found" (ssh-exec transport):**
The CLI tool isn't in the SSH session's PATH. Use an absolute path for the command (e.g., `/usr/local/bin/claude` instead of `claude`). Deep scan does this automatically.

**Host key verification failed:**
Society verifies SSH host keys against `~/.ssh/known_hosts`. If the remote host isn't in your known_hosts file, SSH in manually first to accept the key:

```bash
ssh user@server echo ok
```

**"knownhosts: key mismatch" (especially with Tailscale SSH):**
Society uses Go's `knownhosts` library, which checks the **key type** the server presents. Some SSH servers (notably Tailscale SSH) present an **ecdsa** key by default, but `ssh-keyscan -t ed25519` only saves the ed25519 key. To fix, scan all key types:

```bash
ssh-keygen -R my-server                    # remove old entries
ssh-keyscan my-server >> ~/.ssh/known_hosts  # save all key types
```

**"reading key" error:**
Check that `key_path` points to a valid, unencrypted private key and the file is readable.
