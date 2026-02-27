---
title: A2A Protocol
description: The Agent-to-Agent wire protocol
---

Society implements a subset of the [Agent-to-Agent (A2A) protocol](https://google.github.io/A2A/) over JSON-RPC 2.0. Every agent exposes two HTTP endpoints.

## Endpoints

### `GET /.well-known/agent.json`

Returns the agent card — metadata about the agent.

```json
{
  "name": "claude",
  "description": "Claude Code agent",
  "url": "http://localhost:8003",
  "skills": [
    { "id": "code", "name": "Code Assistant" }
  ],
  "capabilities": {
    "streaming": false,
    "pushNotifications": false
  }
}
```

### `POST /`

Accepts JSON-RPC 2.0 requests. Only the `tasks/send` method is supported.

## Request format

```json
{
  "jsonrpc": "2.0",
  "id": "task-123",
  "method": "tasks/send",
  "params": {
    "id": "task-123",
    "message": {
      "role": "user",
      "parts": [
        { "type": "text", "text": "Hello, agent" }
      ]
    }
  }
}
```

The `id` in both the JSON-RPC envelope and `params` should match. This ID is used as the thread/task identifier for conversation continuity.

## Response format

```json
{
  "jsonrpc": "2.0",
  "id": "task-123",
  "result": {
    "id": "task-123",
    "status": {
      "state": "completed",
      "message": ""
    },
    "messages": [],
    "artifacts": [
      {
        "parts": [
          { "type": "text", "text": "Hello! I'm the agent." }
        ]
      }
    ]
  }
}
```

## Task states

| State | Meaning |
|-------|---------|
| `submitted` | Task received, not yet started |
| `working` | Task in progress |
| `completed` | Task finished successfully |
| `failed` | Task failed |
| `cancelled` | Task was cancelled |

## Message structure

Messages have a `role` (`user` or `agent`) and a list of `parts`:

```json
{
  "role": "user",
  "parts": [
    { "type": "text", "text": "the message content" }
  ]
}
```

Currently only `text` parts are supported.

## Artifacts

Artifacts are the agent's output, separate from messages. Each artifact has a list of parts:

```json
{
  "artifacts": [
    {
      "name": "response",
      "parts": [
        { "type": "text", "text": "the response content" }
      ]
    }
  ]
}
```

## Errors

Unknown methods return a JSON-RPC error:

```json
{
  "jsonrpc": "2.0",
  "id": "task-123",
  "error": {
    "code": -32601,
    "message": "method not found: unknown/method"
  }
}
```

## Limits

- Maximum request body: 1 MB
- HTTP timeout: 30 seconds (configurable per transport)
