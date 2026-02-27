---
title: Daemon Mode
description: Run multiple agents in one process
---

The daemon starts all your agents from a single command, either in the foreground (for development) or as a background process.

## Start in the background

```bash
society daemon start
```

```
Daemon started (PID 12345)
  echo on :8001
  greeter on :8002
  claude on :8003
```

The daemon re-executes the society binary as a child process. Logs go to `~/.society/daemon.log` and state is tracked in `~/.society/daemon.json`.

## Start in the foreground

```bash
society daemon run
```

Same behavior, but logs go to stdout and Ctrl+C stops everything. Useful during development.

## Start specific agents only

```bash
# By name
society daemon start echo claude

# Or run in foreground
society daemon run echo greeter
```

Names must match the `name` field in your YAML configs.

## Custom agents directory

By default, the daemon looks for `*.yaml` and `*.yml` files in the `agents/` directory. Override with `--agents`:

```bash
society daemon start --agents /etc/society/agents
society daemon run --agents ~/my-agents
```

## Check status

```bash
society daemon status
```

```
Daemon: running (uptime: 2h 15m) [PID 12345]
  echo on :8001
  greeter on :8002
  claude on :8003

3 agents active
```

## Stop the daemon

```bash
society daemon stop
```

```
Sent SIGTERM to daemon (PID 12345)
Daemon stopped
```

The daemon handles SIGTERM gracefully — it shuts down all HTTP servers and cleans up the PID file.

## Port conflicts

The daemon checks for port conflicts before starting. If two agent configs use the same port, you get an immediate error:

```bash
society daemon start
# error: port 8001 conflict: echo and my-other-agent
```

## Logs

Background daemon logs are written to `~/.society/daemon.log`. Check them if the daemon fails to start or agents crash:

```bash
tail -f ~/.society/daemon.log
```
