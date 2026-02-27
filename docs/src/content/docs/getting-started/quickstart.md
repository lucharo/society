---
title: Quickstart
description: Run agents locally, in Docker, and on a remote machine in 10 minutes
---

This guide walks through a real setup: a **local machine** running agents, a **Docker container** with an agent inside, and a **remote server** reachable over SSH. By the end, agents on all three can talk to each other.

## 1. Start local agents

Society ships with example agent configs in the `agents/` directory. Start them all:

```bash
# Foreground (see logs, Ctrl+C to stop)
society daemon run

# Or background
society daemon start
```

```
Daemon running (PID 12345)
  echo on :8001
  greeter on :8002
  claude on :8003
3 agents starting
```

Test them:

```bash
society send echo "hello"
#  Thread a1b2c3d4-...
#  Status: completed
#  hello

society send greeter "world"
#  Thread e5f6g7h8-...
#  Status: completed
#  Hello! You said: world
```

## 2. Add an agent in Docker

Build and run a container with an agent inside:

```bash
# Build the society image
docker build -t society .

# Run an echo agent in a container
docker run -d --name echo-agent society run --config /etc/society/agents/echo.yaml
```

Register the Docker agent in your local registry:

```bash
society onboard
```

```
Agent name: docker-echo
Description: Echo agent in Docker
Transport [http/ssh/docker/stdio] (http): docker
Container name or ID: echo-agent
Agent port (8080): 8001
Docker network (optional):
Skills (comma-separated IDs, or empty):

Added "docker-echo" to registry
```

Test it:

```bash
society ping docker-echo
#  docker | docker-echo | 12ms

society send docker-echo "hello from the host"
#  Thread ...
#  Status: completed
#  hello from the host
```

## 3. Add an agent on a remote server

On your remote server, copy the binary and start an agent:

```bash
# On your local machine — cross-compile and copy
GOOS=linux GOARCH=amd64 go build -o /tmp/society-linux ./cmd/society
scp /tmp/society-linux user@server:/usr/local/bin/society

# On the server — start agents
society daemon start --agents /path/to/agents
```

Back on your local machine, register the remote agent:

```bash
society onboard
```

```
Agent name: server-claude
Description: Claude Code on my server
Transport [http/ssh/docker/stdio] (http): ssh
SSH host: server
SSH user: user
SSH key path: ~/.ssh/id_ed25519
SSH port (22):
Agent port on remote host (8080): 8003
Skills (comma-separated IDs, or empty): code

Added "server-claude" to registry
```

Test it:

```bash
society ping server-claude
#  ssh | server-claude | code | 89ms

society send server-claude "write a hello world in Python"
#  Thread ...
#  Status: completed
#  print("Hello, World!")
```

## 4. Multi-turn conversations

Use `--thread` to continue a conversation:

```bash
society send --thread my-session server-claude "write a fibonacci function"
# ... returns the function ...

society send --thread my-session server-claude "now add memoization"
# ... modifies the function, remembers the context ...

society send --thread my-session server-claude "write tests for it"
# ... writes tests referencing the memoized version ...
```

The thread ID is passed as the task ID in the A2A protocol. For `exec` handler agents (like Claude), it also resumes the underlying session, so the agent has full conversation history.

## 5. See all your agents

```bash
society list
```

```
NAME             TRANSPORT  ENDPOINT                          SKILLS
echo             http       http://localhost:8001              echo
greeter          http       http://localhost:8002              greet
claude           http       http://localhost:8003              code, general
docker-echo      docker     docker://echo-agent:8001
server-claude    ssh        ssh://user@server:22→:8003        code
```

## Next steps

- [Create custom agents](/guides/creating-agents/) with the `exec` handler to wrap any CLI tool
- [Connect more machines](/guides/connecting-machines/) with Tailscale and SSH
- [Expose agents via MCP](/guides/mcp/) for Claude Desktop or Cursor
- Read about each [transport](/transports/http/) in detail
