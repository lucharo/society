---
title: Registry
description: How the agent registry works
---

The registry is a JSON file that maps agent names to their connection details. Every `send`, `ping`, and MCP call looks up the target agent in the registry.

## File location

Default: `registry.json` in the current directory.

Override with:
- `--registry <path>` flag (before the subcommand)
- `SOCIETY_REGISTRY` environment variable

## Format

```json
{
  "agents": [
    {
      "name": "echo",
      "url": "http://localhost:8001",
      "description": "Echoes messages back",
      "version": "1.0.0",
      "skills": [
        { "id": "echo", "name": "Echo", "description": "Echoes input" }
      ],
      "capabilities": {
        "streaming": false,
        "pushNotifications": false
      },
      "transport": null
    },
    {
      "name": "server-claude",
      "url": "http://localhost:8003",
      "description": "Claude on remote server",
      "transport": {
        "type": "ssh",
        "config": {
          "host": "server",
          "user": "deploy",
          "key_path": "/home/you/.ssh/id_ed25519",
          "port": "22",
          "forward_port": "8003"
        }
      }
    }
  ]
}
```

## Agent card fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Unique agent name |
| `url` | string | Yes | Agent URL (used for HTTP transport or display) |
| `description` | string | No | Human-readable description |
| `version` | string | No | Agent version |
| `skills` | array | No | List of `{id, name, description}` |
| `capabilities` | object | No | `{streaming, pushNotifications}` booleans |
| `transport` | object | No | Transport config (`null` = HTTP) |

## Transport config

When `transport` is `null` or absent, HTTP transport is used with the `url` field directly.

Otherwise:

```json
{
  "type": "ssh|docker|stdio",
  "config": { ... }
}
```

See [Transports](/transports/http/) for config keys per type.

## Operations

### Add agents

```bash
society onboard          # interactive
society discover <url>   # from running agent
society import <file>    # bulk import
```

### View agents

```bash
society list
```

### Remove agents

```bash
society remove <name>
```

### Export/import

```bash
society export --output backup.json
society import backup.json
```

Import handles conflicts interactively: overwrite, skip, or rename.

## Automatic creation

If the registry file doesn't exist when society runs, it creates an empty one automatically. You don't need to create it manually.

## MCP and the registry

The MCP server reloads the registry on every `tools/list` call. You can add or remove agents while the MCP server is running — clients pick up changes on their next tool list refresh.
