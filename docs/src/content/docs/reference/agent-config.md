---
title: Agent Config
description: YAML agent configuration reference
---

Agent configs are YAML files that define how an agent runs. They're used by `society run` and `society daemon`.

## Full schema

```yaml
name: string          # Required. Unique agent name.
description: string   # Optional. Human-readable description.
port: integer         # Optional. HTTP listen port (1-65535).
handler: string       # Required. One of: echo, greeter, exec.
system_prompt: string # Optional. System prompt for agent identity/role.

backend:              # Required when handler is "exec".
  command: string     # Required. Executable name or path.
  args: [string]      # Optional. Arguments prepended to each invocation.
  session_flag: string # Optional. Flag for session ID on first message.
  resume_flag: string  # Optional. Flag for session ID on follow-up messages.
  system_prompt_flag: string # Optional. Flag to pass the system prompt.
  env: [string]       # Optional. Environment variables (KEY=VALUE format).

skills:               # Optional. Capabilities advertised in the agent card.
  - id: string
    name: string
    description: string  # Optional.
```

## Handlers

### `echo`

Returns the input message unchanged.

```yaml
name: echo
port: 8001
handler: echo
```

### `greeter`

Prepends "Hello! You said: " to each message.

```yaml
name: greeter
port: 8002
handler: greeter
```

### `exec`

Runs an external command for each message. The command receives the message text as an argument and its stdout becomes the response.

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

#### Session continuity

When `session_flag` and `resume_flag` are set, the exec handler maintains conversation state:

- **First message** in a thread: appends `<session_flag> <session-id>`
- **Follow-up messages**: appends `<resume_flag> <session-id>`

The session ID is stored in `~/.society/threads/<thread-id>.json`.

#### Output parsing

The exec handler tries to parse stdout as `{"result": "<text>"}` (the JSON format Claude Code uses with `--output-format json`). If that fails, it uses raw stdout as the response text.

## Examples

### Claude Code

```yaml
name: claude
description: Claude Code agent
port: 8003
handler: exec
backend:
  command: claude
  args: ["-p", "--output-format", "json"]
  session_flag: "--session-id"
  resume_flag: "--resume"
skills:
  - id: code
    name: Code Assistant
  - id: general
    name: General
```

### Ollama

```yaml
name: llama
description: Local Llama via Ollama
port: 8010
handler: exec
backend:
  command: ollama
  args: ["run", "llama3.2"]
```

### Custom script

```yaml
name: summarizer
description: Document summarizer
port: 8020
handler: exec
backend:
  command: python3
  args: ["tools/summarize.py", "--model", "gpt-4o"]
  env:
    - OPENAI_API_KEY=sk-xxx
skills:
  - id: summarize
    name: Summarize
```

## System prompts

The `system_prompt` field gives an agent identity and role context. The prompt is passed to the backend CLI via `system_prompt_flag`.

```yaml
name: code-reviewer
port: 8005
handler: exec
system_prompt: "You are a senior code reviewer. Be concise and focus on bugs and security issues."
backend:
  command: claude
  args: ["-p", "--output-format", "json"]
  system_prompt_flag: "--system-prompt"
```

Known defaults for `system_prompt_flag` (used when the field is omitted):

| CLI | Default flag |
|-----|-------------|
| `claude` | `--system-prompt` |
| `happy` | `--system-prompt` |
| `goose` | `--system` |

If your CLI doesn't support system prompts, omit both fields.

## Autonomous mode defaults

When `society onboard --deep` detects CLI tools on remote hosts, it assigns default arguments so agents can run unattended (nobody is there to approve prompts). These defaults are set in the registry entry's `args` field:

| CLI | Default args |
|-----|-------------|
| `claude` | `-p --output-format json --dangerously-skip-permissions` |
| `codex` | `--quiet --full-auto` |
| `aider` | `--yes-always` |
| `droid` | `--auto high` |
| `opencode` | `--quiet` |

You can override these after onboarding by editing `~/.society/registry.json`.

## File location

The daemon discovers all `*.yaml` and `*.yml` files in the agents directory (default: `agents/`). Override with `--agents <dir>`.
