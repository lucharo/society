---
title: STDIO Transport
description: Spawn agents as subprocesses and communicate over stdin/stdout
---

The STDIO transport launches an agent as a subprocess and communicates via newline-delimited JSON-RPC over its stdin and stdout. No network involved.

## When to use

- Agents that should only run on demand (spawned per request)
- Embedding agents without running a server
- Testing agents locally without opening ports
- Wrapping tools that read from stdin and write to stdout

## Registration

```bash
society onboard
```

```
Agent name: local-claude
Description: Claude Code via stdio
Transport [http/ssh/docker/stdio] (http): stdio
Command: society
Args (space-separated, optional): run --config agents/claude.yaml --stdio
```

## Registry entry

```json
{
  "name": "local-claude",
  "url": "stdio://society",
  "transport": {
    "type": "stdio",
    "config": {
      "command": "society",
      "args": "run --config agents/claude.yaml --stdio"
    }
  }
}
```

## Config reference

| Key | Required | Default | Description |
|-----|----------|---------|-------------|
| `command` | Yes | — | Executable to run (must be on PATH) |
| `args` | No | — | Space-separated arguments |

## How it works

1. Verifies the command exists on PATH
2. Starts the subprocess with stdin/stdout pipes
3. Writes a JSON-RPC request as a single line to the process's stdin
4. Reads the JSON-RPC response from the process's stdout
5. Routes responses by JSON-RPC `id` field (supports concurrent requests)
6. On close: closes stdin, waits up to 5 seconds, then kills the process

Subprocess stderr is logged at DEBUG level — it doesn't interfere with the JSON-RPC protocol.

## Example: stdio-only agent

Run an agent in stdio mode (no HTTP server):

```bash
society run --config agents/echo.yaml --stdio
```

Then pipe messages to it:

```bash
echo '{"jsonrpc":"2.0","id":"1","method":"tasks/send","params":{"id":"1","message":{"role":"user","parts":[{"type":"text","text":"hello"}]}}}' | society run --config agents/echo.yaml --stdio
```

## Multiplexing

The STDIO transport supports multiple concurrent requests. Each request gets a unique JSON-RPC `id`, and responses are routed back to the correct caller by matching IDs. This means you can send multiple messages to a stdio agent without waiting for each response.
