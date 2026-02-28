# society

> Agent-to-Agent orchestration over JSON-RPC 2.0

Connect AI agents across machines, containers, and networks. One CLI to run, manage, and talk to them all.

![society banner](docs/src/assets/banner.png)

## How it works

![Architecture: four layers — Interfaces (MCP Server for Claude Code/Cursor, CLI for society send/ping), Society Core (Registry, Client, Thread Manager), Transports (HTTP, SSH Tunnel, Docker Socket, STDIO Subprocess), and Agents (local, remote via SSH, containers, CLI tools like ollama and claude)](docs/src/assets/architecture.png)

Society implements the [A2A protocol](https://a2a-protocol.org) (JSON-RPC 2.0 over HTTP). Agents expose `GET /.well-known/agent-card.json` for discovery and `POST /` with `tasks/send` for messaging. Society adds transport abstraction on top — SSH tunnels, Docker sockets, and STDIO subprocesses — so agents can live anywhere.

## Install

**Quick install** (macOS / Linux):

```bash
curl -fsSL https://society.luischav.es/install.sh | sh
```

**From source** (requires Go 1.24+):

```bash
git clone https://github.com/lucharo/society.git
cd society
go build -o society ./cmd/society

# Optional: move to a directory on your PATH
mv society ~/.local/bin/  # ensure ~/.local/bin is in your PATH
```

**Cross-compile** for a remote server:

```bash
GOOS=linux GOARCH=amd64 go build -o society-linux ./cmd/society
scp society-linux user@server:~/.local/bin/society
```

## Quick start

### 1. Start agents

```bash
# Detect and register agents
society onboard

# Start all agents from agents/ directory
society daemon start

# Or run in foreground
society daemon run
```

### 2. Talk to them

```bash
society send echo "hello"
society send greeter "world"
society send claude "write a fibonacci function in Python"
```

### 3. Multi-turn conversations

```bash
society send --thread session-1 claude "write a fibonacci function"
society send --thread session-1 claude "add memoization"
society send --thread session-1 claude "write tests for it"
```

### 4. Connect a remote agent

```bash
# Deep scan finds CLI tools on SSH hosts automatically
society onboard --deep
# ✓ Found 2 remote CLI tools via SSH: arch-claude, arch-codex

# Or register manually
society onboard --manual
# → name: server-claude
# → transport: ssh-exec
# → host: my-server
# → command: claude

# Now talk to it
society send server-claude "hello from my laptop"
```

### 5. Use as MCP tools

Expose your agents to Claude Desktop, Cursor, or Claude Code:

```bash
# Add to your project's .mcp.json:
cat > .mcp.json << 'EOF'
{
  "mcpServers": {
    "society": {
      "command": "society",
      "args": ["mcp"]
    }
  }
}
EOF
```

Every registered agent becomes a tool: `send_echo`, `send_claude`, `send_server_claude`, etc. The MCP server reloads the registry on each `tools/list` call, so you can add agents without restarting.

## Transports

| Transport | Use case |
|-----------|----------|
| **HTTP** | Local agents, same network |
| **SSH** | Remote servers with A2A daemon (tunnel) |
| **SSH Exec** | Remote CLI tools (claude, codex) over SSH — no daemon needed |
| **Docker** | Agents in containers |
| **STDIO** | On-demand subprocess agents |

## Agent config

Agents are YAML files in the `agents/` directory:

```yaml
name: claude
description: Claude Code agent
port: 8003
handler: exec
backend:
  command: claude
  args: ["-p", "--output-format", "json"]
  session_flag: "--session-id"
  resume_flag: "--resume"
```

The `exec` handler wraps any CLI tool as an agent. Built-in handlers: `echo`, `greeter`.

## Commands

```
society onboard [--manual] [--deep] Auto-detect and register agents
society list                       List all agents
society send <name> <message>      Send a message [--thread <id>]
society ping <name>                Health-check an agent
society daemon start               Start all agents in background
society daemon stop                Stop the daemon
society daemon status              Show running agents
society mcp                        Start MCP server (stdio)
society discover <url>             Discover agent from A2A endpoint
society import <file>              Import agents from JSON
society export                     Export registry
society update                     Update to latest release
society version                    Print current version
society skill install              Install Claude Code skills
```

## Docs

Full documentation: [society.luischav.es](https://society.luischav.es)

## License

MIT
