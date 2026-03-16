# Agent Chat Bridge

Multi-bot Telegram-to-Claude Code bridge. A single process runs multiple Telegram bots, each acting as a separate agent with its own personality, model, tool restrictions, and user whitelist.

## How it works

Each Telegram bot receives private messages from authorized users. Text, voice, audio, documents, photos, and videos are forwarded to Claude Code CLI. Responses stream back via progressive Telegram message edits.

## Requirements

- Go 1.22+
- [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code) v2.1+
- Telegram bot tokens from [BotFather](https://t.me/BotFather)

## Quick start

1. Copy example config:
   ```bash
   cp configs/config.yaml.example configs/config.yaml
   ```

2. Edit `configs/config.yaml` -- set `claude.binary` path, add your bot tokens and users.

3. Build and run:
   ```bash
   make run
   ```

## Running

```bash
make build          # compile binary
make run            # build and run (foreground)
make start          # build and run in background (PID file + log file)
make stop           # stop background process (SIGTERM, graceful shutdown)
make restart        # stop + start
make logs           # tail log file (JSON via jq)
```

Custom config path:
```bash
./agent-chat-bridge --config /path/to/config.yaml
```

## Configuration

Config file: `configs/config.yaml` (gitignored, tokens stored directly).
Example: `configs/config.yaml.example`.

### Minimal config

```yaml
claude:
  binary: "/usr/local/bin/claude"

telegram_bots:
  mybot:
    token: "123456:ABC-your-bot-token"
    users:
      123456789:
        working_dir: "/home/user/projects"
```

### Bot with system prompt

```yaml
telegram_bots:
  obsidian:
    token: "123456:ABC-your-bot-token"
    model: "opus"
    permission_mode: "bypassPermissions"
    append_system_prompt: "You work with an Obsidian vault."
    users:
      123456789:
        working_dir: "/home/user/vault"
```

### Bot with agent mode

Agent mode launches Claude in a constrained role with its own system prompt and optional tool restrictions:

```yaml
telegram_bots:
  translator:
    token: "654321:DEF-your-bot-token"
    model: "sonnet"
    permission_mode: "plan"
    agent:
      name: "translator"
      description: "Translates text between languages"
      prompt: "You are a translator. Only translate the text provided by the user."
      tools: []
    users:
      123456789:
        working_dir: "/home/user/translations"
```

`agent` and `append_system_prompt` are mutually exclusive.

### Agent tool restrictions

The `agent.tools` field controls which Claude Code tools are available:

| Value | Meaning |
|-------|---------|
| not specified | All default tools (Bash, Edit, Read, etc.) |
| `tools: []` | All tools disabled -- text-only mode |
| `tools: [Read, Glob]` | Only the listed tools available |

### Permission modes

The `permission_mode` field controls how Claude Code handles tool approvals:

| Mode | Description |
|------|-------------|
| `bypassPermissions` | No approval needed for any tool (default) |
| `plan` | Claude proposes a plan, tools run without approval |
| `acceptEdits` | Edits require approval, other tools run freely |
| `default` | All tool usage requires approval |

### Config reference

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `claude.binary` | yes | -- | Path to Claude Code CLI |
| `claude.timeout_minutes` | no | 10 | Per-request timeout |
| `telegram_bots.<name>.token` | yes | -- | Telegram Bot API token |
| `telegram_bots.<name>.model` | no | CLI default | Model alias or full name |
| `telegram_bots.<name>.permission_mode` | no | bypassPermissions | See table above |
| `telegram_bots.<name>.append_system_prompt` | no | -- | Extra system prompt |
| `telegram_bots.<name>.agent` | no | -- | Agent definition (see above) |
| `telegram_bots.<name>.sessions` | no | `<name>_sessions.json` | Session persistence file |
| `users.<id>.working_dir` | yes | -- | CWD for Claude process |
| `users.<id>.voice_dir` | no | `voice_inbox` | Voice message storage (relative to working_dir) |
| `users.<id>.files_dir` | no | `files_inbox` | File storage (relative to working_dir) |

## Bot commands

| Command   | Description                        |
|-----------|------------------------------------|
| `/start`  | Welcome message and command list   |
| `/help`   | Same as /start                     |
| `/new`    | Reset session, start fresh         |
| `/stop`   | Cancel active request              |
| `/status` | Show active request and session    |

## Running as a macOS service

Install as a launchd service to auto-start on login and restart on crash:

```bash
./scripts/service.sh install    # build, install to ~/.local/bin, create launchd plist
./scripts/service.sh reload     # rebuild binary, restart (picks up config changes)
./scripts/service.sh status     # show service status
./scripts/service.sh logs       # tail service logs
./scripts/service.sh uninstall  # stop service, remove plist and binary
```

Locations:
- Binary: `~/.local/bin/agent-chat-bridge`
- Config: `~/.config/agent-chat-bridge/config.yaml` (copied from `configs/config.yaml`)
- Logs: `~/.local/share/agent-chat-bridge/logs/`

Edit `configs/config.yaml` in the project, then run `reload` to apply.

## Development

```bash
make build       # compile binary
make test        # run unit tests
make test-race   # run tests with race detector
make lint        # go vet
```

## License

[MIT](LICENSE)
