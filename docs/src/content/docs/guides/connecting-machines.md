---
title: Connecting Machines
description: Make agents on different machines aware of each other
---

Society agents don't automatically discover each other. The **registry** is what makes agents aware of one another — it maps agent names to their connection details (URL + transport config).

This guide walks through connecting agents across a local machine and a remote server step by step.

## The setup

```
┌──────────────────┐            ┌──────────────────┐
│   Local Machine   │            │   Remote Server   │
│                   │            │                   │
│  echo     :8001   │◄──SSH────▸│  claude    :8003  │
│  greeter  :8002   │  tunnel   │  echo      :8001  │
│                   │            │                   │
│  registry.json    │            │  registry.json    │
└──────────────────┘            └──────────────────┘
```

Each machine has its own registry. To talk to a remote agent, you register it locally with the right transport config.

## Step 1: Start agents on both machines

**On the remote server:**

```bash
# Copy binary (if not already installed)
scp society user@server:~/.local/bin/society

# SSH in and start agents
ssh user@server
cd /path/to/society
society daemon start
```

**On the local machine:**

```bash
society daemon start
```

## Step 2: Register remote agents locally

Use `onboard` to add the remote agent to your local registry:

```bash
society onboard
```

```
Agent name: server-claude
Description: Claude on remote server
Transport [http/ssh/docker/stdio] (http): ssh
SSH host: server
SSH user: user
SSH key path: ~/.ssh/id_ed25519
SSH port (22): 22
Agent port on remote host (8080): 8003
Skills (comma-separated IDs, or empty): code, general

Added "server-claude" to registry
```

Now you can send messages to it:

```bash
society send server-claude "what OS are you running on?"
```

The message flows: local CLI -> SSH tunnel -> server:8003 -> Claude Code -> response back.

## Step 3: Register local agents on the remote (optional)

If you want the remote server to talk back to your local agents, SSH into the server and register them there:

```bash
ssh user@server
cd /path/to/society
society onboard
```

```
Agent name: laptop-echo
Transport: ssh
SSH host: laptop   # or IP/hostname
SSH user: you
SSH key path: ~/.ssh/id_ed25519
Agent port: 8001
```

## Using Tailscale

If both machines are on [Tailscale](https://tailscale.com), you can use Tailscale hostnames directly:

```bash
society onboard
```

```
Agent name: arch-claude
Transport: ssh
SSH host: arch           # Tailscale hostname
SSH user: luis
SSH key path: ~/.ssh/id_ed25519
Agent port: 8003
```

Or if the Tailscale network allows direct HTTP:

```bash
society onboard
```

```
Agent name: arch-claude
Transport: http
URL: http://arch:8003    # Tailscale resolves this
```

The SSH transport is preferred because it works even when the remote machine's ports aren't directly exposed.

## Import/export registries

Instead of manually onboarding each agent, you can export a registry from one machine and import it on another.

**On the server:**

```bash
society export --output /tmp/server-agents.json
```

**Copy and import on local:**

```bash
scp user@server:/tmp/server-agents.json .
society import server-agents.json
```

The import command will prompt for transport config for each agent, since transport details differ per machine (paths, hostnames, etc.).

## Discover agents from a running server

If an agent is already running and reachable, you can discover it by URL:

```bash
society discover http://server:8001
```

```
Found agent:
  Name: echo
  Description: Echoes messages back
  Skills: echo

Add to registry? [Y/n] y
Transport [http/ssh/docker/stdio]: http

Added "echo" to registry
```

This fetches the agent card from `/.well-known/agent.json` and registers it.

## Verifying connectivity

```bash
# Check what's registered
society list

# Health-check a specific agent
society ping server-claude
```

The `ping` command opens the transport, sends a test message, and reports latency. If it fails, you know the transport config or the remote agent is the issue.
