# Changelog

## v0.3.0

### New Features

- **SSH-exec transport** — Message CLI agents (claude, codex, aider, etc.) on remote hosts via SSH without installing Society remotely. Deep scan auto-detects CLIs via `command -v` and common install paths.
- **Tailscale peer discovery** — `society onboard` discovers Tailscale peers via `tailscale status --json`. Deep scan probes peers for A2A agents over HTTP and CLI tools over SSH.
- **Deep discovery (`--deep` flag)** — Probes SSH hosts and Docker containers for live A2A agents and installed CLI tools. Verified agents skip the manual port prompt during onboarding.
- **System prompt support** — Agents can define a `system_prompt` in YAML config, passed to the backend CLI via a configurable flag. Auto-detected for claude, happy, and goose.
- **Autonomous mode defaults** — Known CLI tools get safe unattended flags during onboard (e.g., `--dangerously-skip-permissions` for claude, `--full-auto` for codex).
- **SSH host deduplication** — Multiple SSH routes to the same machine are grouped during onboard, with a sub-prompt to pick the preferred route.

### Improvements

- **Onboard UX overhaul** — Auto-detect is now the default (use `--manual` for the wizard). Transport details shown per agent, post-registration summary table, next-step hints.
- **CLI command polish** — `list` uses bold headers and cleaner endpoint display. `ping` shows agent name with latency. `send` outputs response text directly. `help`/`-h`/`--help` works.
- **A2A spec alignment** — Agent card endpoint moved to `/.well-known/agent-card.json` per spec. Legacy path preserved for backwards compatibility.
- **Registry moved to `~/.society/`** — Default path is now `~/.society/registry.json`. Parent directory auto-created. `--registry` flag and `SOCIETY_REGISTRY` env var still work.

### Security

- **SSH host key verification** — Replaced `InsecureIgnoreHostKey` with `known_hosts`-based verification across all SSH connections.
- **SSH-exec hardening** — Shell-escape all command args, cap output at 10MB, guard against double `Open()`.
- **Update command hardening** — SHA256 checksum verification, atomic binary replacement, size caps, hash format validation.
- **Docker API sanitization** — `url.PathEscape` for container names.

### Bug Fixes

- Fix `UserHomeDir` error handling in scan progress messages.

### Documentation

- Document trust model and security boundary.
- Expand SSH transport docs: tunnel, exec, Tailscale SSH, macOS setup, known_hosts troubleshooting.
- Document system prompts and autonomous mode defaults.
- Add `--deep` flag to CLI reference.

## v0.2.0

Initial public release.
