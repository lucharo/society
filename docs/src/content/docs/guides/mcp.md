---
title: MCP Integration
description: Expose agents as MCP tools for Claude Desktop and Cursor
---

Society can run as an [MCP (Model Context Protocol)](https://modelcontextprotocol.io) server, exposing every registered agent as a tool. This lets Claude Desktop, Cursor, Claude Code, or any MCP client talk to your agents natively.

## How it works

```bash
society mcp
```

This starts a JSON-RPC 2.0 server on stdio. Each agent in your registry becomes an MCP tool:

| Agent | MCP Tool |
|-------|----------|
| `echo` | `send_echo` |
| `greeter` | `send_greeter` |
| `claude` | `send_claude` |
| `arch-claude` | `send_arch_claude` |

Hyphens in agent names become underscores in tool names.

Each tool accepts:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `message` | string | Yes | The message to send |
| `thread_id` | string | No | Thread ID to continue a conversation |

## Configure in Claude Code

Add to your project's `.mcp.json`:

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

Or with a custom registry path:

```json
{
  "mcpServers": {
    "society": {
      "command": "society",
      "args": ["--registry", "/path/to/registry.json", "mcp"]
    }
  }
}
```

After restarting Claude Code, your agents appear as tools: `send_echo`, `send_claude`, etc.

## Configure in Claude Desktop

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS):

```json
{
  "mcpServers": {
    "society": {
      "command": "/path/to/society",
      "args": ["--registry", "/path/to/registry.json", "mcp"]
    }
  }
}
```

## Multi-turn conversations

MCP tools support the `thread_id` parameter. When an MCP client passes the same thread ID across multiple tool calls, the underlying agent maintains conversation context:

```
Tool: send_claude
Args: { "message": "write a fibonacci function", "thread_id": "session-1" }
→ returns the function

Tool: send_claude
Args: { "message": "add memoization", "thread_id": "session-1" }
→ modifies the function, remembers context
```

## Error handling

Agent errors are returned as MCP tool results with `isError: true`, not as JSON-RPC protocol errors. This means the MCP client can display the error message to the user and decide what to do:

```json
{
  "content": [{ "type": "text", "text": "Error: connection refused" }],
  "isError": true
}
```

## Dynamic registry

The MCP server reloads the registry on every `tools/list` request. This means you can add or remove agents from the registry while the MCP server is running — the next time the client refreshes its tool list, it picks up the changes.
