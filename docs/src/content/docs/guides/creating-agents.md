---
title: Creating Agents
description: Define agents with YAML configs and custom handlers
---

An agent is a YAML config file that tells society what handler to use and which port to listen on. Society discovers agent configs from a directory (default: `agents/`).

## Agent config format

```yaml
name: my-agent
description: What this agent does
port: 8010
handler: exec
backend:
  command: my-tool
  args: ["--flag", "value"]
  session_flag: "--session"
  resume_flag: "--resume"
  env:
    - API_KEY=sk-xxx
skills:
  - id: summarize
    name: Summarize
    description: Summarizes documents
```

### Fields

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Unique agent name |
| `description` | No | Human-readable description |
| `port` | No | HTTP listen port (1-65535) |
| `handler` | Yes | `echo`, `greeter`, or `exec` |
| `backend` | Only for `exec` | External command configuration |
| `skills` | No | List of capabilities the agent advertises |

## Built-in handlers

### echo

Returns the input message unchanged. Useful for testing.

```yaml
name: echo
port: 8001
handler: echo
```

### greeter

Prepends "Hello! You said: " to every message. Another test handler.

```yaml
name: greeter
port: 8002
handler: greeter
```

### exec

The real workhorse. Wraps any CLI tool as an agent. The tool receives the message text on its arguments and should output a response.

```yaml
name: claude
port: 8003
handler: exec
backend:
  command: claude
  args: ["-p", "--output-format", "json"]
  session_flag: "--session-id"
  resume_flag: "--resume"
```

#### How exec works

For each incoming message:

1. Extracts text from the message parts
2. Loads or creates a conversation thread (`~/.society/threads/<id>.json`)
3. Builds the command: `command args... <message-text>`
4. On first message: appends `session_flag <session-id>` if configured
5. On follow-ups: appends `resume_flag <session-id>` instead
6. Runs the command, captures stdout
7. Parses JSON output if possible (looks for `{"result": "..."}` format), otherwise uses raw stdout
8. Saves the exchange to the thread file

#### Examples

**Wrap Claude Code:**

```yaml
name: claude
port: 8003
handler: exec
backend:
  command: claude
  args: ["-p", "--output-format", "json"]
  session_flag: "--session-id"
  resume_flag: "--resume"
```

**Wrap a custom script:**

```yaml
name: translator
port: 8010
handler: exec
backend:
  command: python3
  args: ["translate.py"]
  env:
    - DEEPL_API_KEY=your-key
```

**Wrap any LLM CLI:**

```yaml
name: ollama-llama
port: 8020
handler: exec
backend:
  command: ollama
  args: ["run", "llama3.2"]
```

## System prompts

Give an agent a role by setting `system_prompt`. The prompt is passed to the backend CLI automatically.

```yaml
name: reviewer
port: 8005
handler: exec
system_prompt: "You are a code reviewer. Focus on bugs and security."
backend:
  command: claude
  args: ["-p", "--output-format", "json"]
```

The flag used to pass the prompt is auto-detected for known CLIs (claude uses `--system-prompt`, goose uses `--system`). For other CLIs, set `backend.system_prompt_flag` explicitly. See [Agent Config Reference](/reference/agent-config/#system-prompts) for details.

## Running agents

Single agent:

```bash
society run --config agents/my-agent.yaml
```

All agents in a directory:

```bash
society daemon run --agents ./agents
```

Specific agents only:

```bash
society daemon run echo claude
```

## STDIO mode

Agents can also run over stdio instead of HTTP, useful for embedding:

```bash
society run --config agents/echo.yaml --stdio
```

In this mode, the agent reads JSON-RPC requests from stdin and writes responses to stdout, one per line.
