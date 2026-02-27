---
title: SSH Transport
description: Reach agents on remote machines through SSH tunnels
---

The SSH transport opens an SSH tunnel to the remote host and forwards a local port to the agent's port. All JSON-RPC traffic flows through the tunnel.

## When to use

- Agents on remote servers you can SSH into
- Machines behind firewalls where ports aren't directly exposed
- Tailscale + SSH for secure cross-network access

## Registration

```bash
society onboard
```

```
Agent name: server-claude
Description: Claude on my server
Transport [http/ssh/docker/stdio] (http): ssh
SSH host: my-server
SSH user: deploy
SSH key path: ~/.ssh/id_ed25519
SSH port (22): 22
Agent port on remote host (8080): 8003
Skills (comma-separated IDs, or empty): code
```

## Registry entry

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

## Config reference

| Key | Required | Default | Description |
|-----|----------|---------|-------------|
| `host` | Yes | — | Remote hostname or IP |
| `user` | Yes | — | SSH username |
| `key_path` | Yes | — | Path to SSH private key |
| `port` | No | `22` | SSH port |
| `forward_port` | No | `8080` | Agent port on the remote host |

## How it works

1. Reads and parses the SSH private key
2. Dials an SSH connection to `host:port`
3. Opens a local TCP listener on a random port
4. Forwards connections through the SSH tunnel to `127.0.0.1:forward_port` on the remote
5. Sends HTTP POST to the local forwarded port
6. Closes tunnel and SSH connection when done

Each `send` command opens a fresh tunnel. The tunnel is short-lived — it's created, used for one request, and torn down.

## Tailscale + SSH

A common pattern is using Tailscale hostnames:

```
SSH host: arch        # Tailscale hostname
SSH user: luis
SSH key path: ~/.ssh/id_ed25519
Agent port: 8003
```

Tailscale handles the networking, SSH handles the tunnel. No need to expose ports publicly.

## Troubleshooting

**"connection refused" after SSH connects:**
The SSH tunnel is working but nothing is listening on the remote port. Make sure the agent is running on the remote machine:

```bash
ssh user@server "ss -tlnp | grep 8003"
```

**"host key verification disabled" warning:**
Society currently uses `InsecureIgnoreHostKey` for SSH. This is logged as a warning. Known hosts verification is planned.

**"reading key" error:**
Check that `key_path` points to a valid private key and the file is readable.
