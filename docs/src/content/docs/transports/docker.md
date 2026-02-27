---
title: Docker Transport
description: Talk to agents running inside Docker containers
---

The Docker transport resolves a container's IP address via the Docker Engine API and sends requests directly to it, no port mapping required.

## When to use

- Agents running inside Docker containers on the same host
- When you don't want to publish container ports to the host

## Registration

```bash
society onboard
```

```
Agent name: docker-echo
Description: Echo agent in a container
Transport [http/ssh/docker/stdio] (http): docker
Container name or ID: echo-agent
Agent port (8080): 8001
Docker network (optional): society-net
Docker socket path (/var/run/docker.sock):
```

## Registry entry

```json
{
  "name": "docker-echo",
  "url": "http://echo-agent:8001",
  "transport": {
    "type": "docker",
    "config": {
      "container": "echo-agent",
      "agent_port": "8001",
      "network": "society-net",
      "socket_path": "/var/run/docker.sock"
    }
  }
}
```

## Config reference

| Key | Required | Default | Description |
|-----|----------|---------|-------------|
| `container` | Yes | — | Container name or ID |
| `agent_port` | No | `8080` | Port the agent listens on inside the container |
| `network` | No | (first available) | Docker network to resolve IP from |
| `socket_path` | No | `/var/run/docker.sock` | Path to Docker socket |

## How it works

1. Calls `GET /containers/<name>/json` on the Docker socket
2. Verifies the container is running
3. Resolves the container's IP address on the specified network (or first available)
4. Sends HTTP POST to `http://<container-ip>:<agent_port>`

## Example: run an agent in Docker

```bash
# Build the image
docker build -t society .

# Create a network
docker network create society-net

# Run an echo agent
docker run -d --name echo-agent --network society-net \
  society run --config /etc/society/agents/echo.yaml

# Register it locally
society onboard
# name: docker-echo, transport: docker, container: echo-agent, port: 8001

# Test
society send docker-echo "hello from outside"
```

## Troubleshooting

**"container not running":**
Check `docker ps` to see if the container is up.

**"no IP found on network":**
The container isn't attached to the specified network. Check with `docker inspect echo-agent`.

**Permission denied on Docker socket:**
Your user needs access to `/var/run/docker.sock`. Either add yourself to the `docker` group or use `socket_path` to point to a rootless Docker socket.
