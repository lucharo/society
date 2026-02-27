---
title: HTTP Transport
description: Direct HTTP connections for local agents
---

The default transport. Sends JSON-RPC 2.0 requests directly to the agent's URL over HTTP.

## When to use

- Agents running on the same machine
- Agents on the same network with accessible ports
- Agents behind a reverse proxy
- Tailscale networks where hosts can reach each other directly

## Registration

```bash
society onboard
```

```
Agent name: my-agent
Transport [http/ssh/docker/stdio] (http): http
Agent URL: http://localhost:8001
```

Or just press Enter for the default `http` transport.

## Registry entry

```json
{
  "name": "my-agent",
  "url": "http://localhost:8001",
  "description": "My local agent"
}
```

No `transport` field needed — HTTP is the default when transport is absent or null.

## Configuration

| Setting | Value |
|---------|-------|
| Timeout | 30 seconds |
| Content-Type | `application/json` |
| Method | POST to `/` |

## Tailscale example

If both machines are on Tailscale and ports are reachable:

```json
{
  "name": "remote-agent",
  "url": "http://arch:8003"
}
```

Tailscale resolves `arch` to the machine's Tailscale IP. No SSH tunnel needed, but the port must be accessible (check your Tailscale ACLs).
