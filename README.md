# Agent Chat Bridge (Telebridge)

Multi-bot Telegram-to-Claude Code bridge. A single process runs multiple Telegram bots, each acting as a separate agent with its own personality, model, and user whitelist.

## How it works

Each Telegram bot receives private messages from authorized users. Text, voice, audio, documents, photos, and videos are forwarded to Claude Code CLI. Responses stream back via progressive Telegram message edits.

## Requirements

- Go 1.22+
- [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code) v2.1+
- Telegram bot tokens from [BotFather](https://t.me/BotFather)

## Quick start

1. Copy example configs:
   ```bash
   cp configs/config.yaml.example configs/config.yaml
   cp .env.example .env
   ```

2. Edit `configs/config.yaml` -- set `claude.binary` path, add your bots and users.

3. Edit `.env` -- set real Telegram bot tokens:
   ```
   TELEBRIDGE_MYBOT_TOKEN=123456:ABC-DEF...
   ```

4. Build and run:
   ```bash
   make run
   ```

## Configuration

See `config.example.yaml` for the full structure. Key points:

- `claude.binary` -- path to Claude Code CLI binary (required)
- Each bot has a `token`, optional `model`, `permission_mode`, and `append_system_prompt`
- Each user is identified by Telegram user ID and needs a `working_dir`
- Bot tokens can be overridden via environment variables: `TELEBRIDGE_<BOTNAME>_TOKEN`

## Bot commands

| Command   | Description                        |
|-----------|------------------------------------|
| `/start`  | Welcome message and command list   |
| `/help`   | Same as /start                     |
| `/new`    | Reset session, start fresh         |
| `/stop`   | Cancel active request              |
| `/status` | Show active request and session    |

## Development

```bash
make build       # build binary
make test        # run tests
make test-race   # run tests with race detector
make lint        # go vet
make start       # build and run in background
make stop        # stop background process
make logs        # tail log file
```

## License

[MIT](LICENSE)
