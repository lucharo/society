---
name: society:setup
description: Set up society agents interactively — detect CLIs, configure transports, start daemon
---

# society:setup

Set up society from scratch by detecting available AI agents, registering them, and starting the daemon.

## Usage

```
/society:setup
```

## Instructions

When the user invokes `/society:setup`:

### 1. Check if society is installed

```bash
which society || echo "not found"
```

If not found, tell the user to install it:
```bash
curl -fsSL https://society.luischav.es/install.sh | sh
```

### 2. Auto-detect agents

Run the smart onboard scanner:
```bash
society onboard --auto
```

This will scan for:
- **CLI tools** in PATH: claude, codex, ollama, aider, etc.
- **Docker containers** with exposed ports
- **SSH hosts** from ~/.ssh/config
- **Running A2A agents** on local ports 8001-8010

Present the detected agents and let the user select which to register.

### 3. Set up MCP integration

If the user is in a project directory, create `.mcp.json`:

```bash
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

### 4. Start the daemon

```bash
society daemon start
```

### 5. Verify

```bash
society list
society daemon status
```

Tell the user they can now use `society send <agent> <message>` or talk to agents via MCP tools.
