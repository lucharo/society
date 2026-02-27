---
title: Introduction
description: What society is and how it works
---

Society is a CLI tool for running and connecting AI agents across machines. It implements the [Agent-to-Agent (A2A) protocol](https://google.github.io/A2A/) over JSON-RPC 2.0, letting agents communicate regardless of where they run — locally, in Docker containers, or on remote servers over SSH.

## What it does

- **Runs agents** from YAML config files as HTTP servers
- **Routes messages** between agents through HTTP, SSH tunnels, Docker networks, or stdio pipes
- **Manages a registry** of known agents with their connection details
- **Exposes agents as MCP tools** for Claude Desktop, Cursor, or any MCP-compatible client
- **Daemon mode** starts multiple agents in one process

## How agents work

Every agent is a JSON-RPC 2.0 server that handles one method: `tasks/send`. You send a message, you get a response. That's it.

Agents are defined by two things:

1. **A handler** that processes messages (echo, greeter, or exec — which wraps any CLI tool)
2. **A transport** that determines how to reach the agent (HTTP, SSH, Docker, or stdio)

The `exec` handler is the most powerful — it lets you wrap any command-line tool as an agent. The default config wraps Claude Code, turning it into a conversational agent that maintains session state across messages.

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────────────┐
│ society CLI  │────▸│   Registry   │────▸│  Transport Layer │
│  or MCP      │     │ registry.json│     │  HTTP/SSH/Docker │
└─────────────┘     └──────────────┘     └────────┬────────┘
                                                   │
                          ┌────────────────────────┼────────────────┐
                          │                        │                │
                    ┌─────▾─────┐          ┌──────▾──────┐  ┌─────▾──────┐
                    │  Local     │          │  Docker     │  │  Remote    │
                    │  Agent     │          │  Agent      │  │  Agent     │
                    │  :8001     │          │  container  │  │  SSH :8003 │
                    └───────────┘          └─────────────┘  └────────────┘
```

## Next steps

- [Install society](/getting-started/installation/) to get the binary
- Follow the [quickstart](/getting-started/quickstart/) to run your first agents and send messages across machines
