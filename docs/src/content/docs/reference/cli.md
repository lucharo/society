---
title: CLI Commands
description: Complete reference for all society commands
---

## Global flags

```
--registry <path>    Registry file path
                     Default: registry.json
                     Env: SOCIETY_REGISTRY
```

The `--registry` flag must appear before the subcommand.

---

## `society onboard`

Auto-detect available agents and register them.

```bash
society onboard            # auto-detect CLIs, Docker, SSH, A2A agents
society onboard --manual   # interactive wizard for manual setup
```

By default, scans for CLIs (claude, codex, ollama, etc.), Docker containers, SSH hosts, and A2A agents on local ports. Presents a numbered list and lets you select which to register.

Use `--manual` for the interactive wizard that prompts for name, description, transport type, and transport-specific config.

---

## `society list`

List all registered agents.

```bash
society list
```

Output columns: `NAME`, `TRANSPORT`, `ENDPOINT`, `SKILLS`

---

## `society remove <name>`

Remove an agent from the registry. Prompts for confirmation.

```bash
society remove my-agent
```

---

## `society ping <name>`

Health-check an agent by sending a test message.

```bash
society ping my-agent
```

Reports: transport type, agent name, skills, and latency in milliseconds.

---

## `society run`

Start a single agent from a config file.

```bash
society run --config <path> [--stdio]
```

| Flag | Description |
|------|-------------|
| `--config` | Path to agent YAML config (required) |
| `--stdio` | Run as stdio agent instead of HTTP server |

---

## `society send`

Send a message to a registered agent.

```bash
society send [--thread <id>] <name> <message>
```

| Flag | Description |
|------|-------------|
| `--thread` | Thread ID to continue a conversation |

The `--thread` flag must come before the agent name. Remaining arguments after the name are joined as the message text.

---

## `society export`

Export the registry as JSON.

```bash
society export [--output <path>]
```

Without `--output`, prints to stdout.

---

## `society import <path-or-url>`

Import agents from a JSON file or URL.

```bash
society import agents-backup.json
society import https://example.com/agents.json
```

Prompts interactively for conflicts: overwrite, skip, or rename.

---

## `society discover <url>`

Discover an agent from an A2A endpoint.

```bash
society discover http://server:8001
```

Fetches the agent card from `/.well-known/agent.json`, displays it, and optionally adds it to the registry with a transport config you choose.

---

## `society mcp`

Start an MCP server on stdio.

```bash
society mcp
```

Exposes each registered agent as an MCP tool (`send_<name>`). See [MCP Integration](/guides/mcp/) for setup details.

---

## `society daemon`

Manage the agent daemon.

### `daemon start [agents...] [--agents <dir>]`

Start agents in the background.

```bash
society daemon start                    # all agents in agents/
society daemon start echo claude        # specific agents
society daemon start --agents ~/agents  # custom directory
```

### `daemon stop`

Stop the running daemon (sends SIGTERM, waits up to 5s).

```bash
society daemon stop
```

### `daemon status`

Show daemon uptime, PID, and running agents.

```bash
society daemon status
```

### `daemon run [agents...] [--agents <dir>]`

Start agents in the foreground (logs to stdout, Ctrl+C to stop).

```bash
society daemon run
society daemon run echo greeter
```

---

## `society update`

Check for a new release and update the binary in place.

```bash
society update
```

Downloads the latest release from GitHub, verifies it matches your OS and architecture, and replaces the current binary. On macOS, applies ad-hoc code signing automatically.

Dev builds (built from source without version ldflags) cannot self-update — use the install script or rebuild from source instead.

---

## `society version`

Print the current version.

```bash
society version
# society v0.1.0
```

---

## `society skill`

Manage Claude Code skills for society.

### `skill install`

Install society skills into `~/.claude/skills/`.

```bash
society skill install
```

### `skill update`

Update previously installed skills to the latest version bundled with your society binary.

```bash
society skill update
```

