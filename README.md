# society

> Agent-to-Agent orchestration over JSON-RPC 2.0

Connect AI agents across machines, containers, and networks. One CLI to run, manage, and talk to them all.

![society banner](docs/src/assets/banner.png)

## Install

```bash
git clone https://github.com/lucharo/society.git
cd society
go build -o society ./cmd/society

# Optional: move to a directory on your PATH
mv society ~/.local/bin/  # ensure ~/.local/bin is in your PATH
```

Cross-compile for a remote server:

```bash
GOOS=linux GOARCH=amd64 go build -o society-linux ./cmd/society
scp society-linux user@server:~/.local/bin/society
```

## Quick start

### 1. Start agents

```bash
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
# Register an agent on a remote server
society onboard
# → name: server-claude
# → transport: ssh
# → host: my-server
# → user: deploy
# → key: ~/.ssh/id_ed25519
# → port: 8003

# Now talk to it
society send server-claude "hello from my laptop"
```

### 5. Expose as MCP tools

Add to `.mcp.json` in your project:

```json
{
  "mcpServers": {
    "society": {
      "command": "./society",
      "args": ["mcp"]
    }
  }
}
```

Every registered agent becomes a tool: `send_echo`, `send_claude`, `send_server_claude`, etc.

## Transports

| Transport | Use case |
|-----------|----------|
| **HTTP** | Local agents, same network |
| **SSH** | Remote servers, Tailscale hosts |
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
society onboard                    Register an agent interactively
society list                       List all agents
society send <name> <message>      Send a message
society ping <name>                Health-check an agent
society daemon start               Start all agents in background
society daemon stop                Stop the daemon
society daemon status              Show running agents
society mcp                        Start MCP server (stdio)
society discover <url>             Discover agent from A2A endpoint
society import <file>              Import agents from JSON
society export                     Export registry
```

## Docs

Full documentation: `cd docs && bun install && bun run dev` → [localhost:4321](http://localhost:4321)

## License

MIT
