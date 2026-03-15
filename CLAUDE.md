# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

jira_instance: vanadis

## Project

Agent Chat Bridge -- multi-bot Telegram-to-Claude Code bridge. A single process runs multiple Telegram bots, each with its own personality (system prompt), model, and user whitelist. Messages (text, voice, audio, documents, photos, videos) are forwarded to Claude Code CLI; responses stream back via progressive Telegram message edits.

## Build & Run

```bash
make build          # go build -o agent-chat-bridge ./cmd/agent-chat-bridge
make run            # build + run foreground (loads .env)
make start          # build + run background (nohup, pidfile, debug)
make stop           # graceful stop via pidfile
make restart        # stop + start
make logs           # tail -f /tmp/agent-chat-bridge.log
```

## Test & Lint

```bash
make test           # go test ./...
make test-race      # go test -race ./...
make test-all       # both
make lint           # go vet ./...
```

Run a single test:
```bash
go test -run TestName ./internal/package/...
```

## Architecture

```
cmd/agent-chat-bridge/main.go   -- entry point, flag parsing, lifecycle
internal/
  bot/                           -- Telegram bot: handlers, commands, auth, streaming, file downloads
  claude/                        -- Claude CLI SDK wrapper: runner (streaming), session persistence
  config/                        -- YAML config loading, defaults, validation
  formatter/                     -- Markdown-to-HTML conversion, message splitting (4096 char limit)
  media/                         -- File saving, filename sanitization, collision resolution, quota (10 GB/user)
specs/                           -- spec.md (functional), ops.md (ops), tests.md (test spec)
```

### Key flows

1. **Message handling**: `bot.Handler` receives Telegram update -> auth check -> download media (if any) -> call `claude.Run()` with streaming -> `bot.Streamer` progressively edits Telegram message -> split final message if >4096 chars.
2. **Session continuity**: `claude.SessionStore` persists session IDs per user in `<bot>_sessions.json`. Resume flag passed to Claude SDK on each run. `/new` command resets session.
3. **Concurrency**: Each bot runs in its own goroutine. Per-user mutex prevents duplicate requests. `/stop` cancels active request via context.

### Configuration

- `configs/config.yaml.example` -- template, tracked in git
- `configs/config.yaml` -- real config with local paths and bot settings, gitignored
- `.env.example` -- env var template, tracked in git
- `.env` -- real tokens, gitignored

Config defines `claude` (binary path, timeout, max_concurrent) and `telegram_bots` map (token, model, permission_mode, system prompt, users with working_dir). Tokens can be overridden via `AGENT_CHAT_BRIDGE_<BOT_NAME>_TOKEN` env vars.

## Dependencies

- `github.com/go-telegram-bot-api/telegram-bot-api/v5` -- Telegram API client
- `github.com/severity1/claude-agent-sdk-go` -- Claude Code CLI SDK (streaming)
- `gopkg.in/yaml.v3` -- config parsing

## Testing patterns

- Standard `testing` package, table-driven tests, `t.Helper()` in helpers
- `TelegramSender` interface for mocking Telegram API
- `httptest.Server` for Telegram API mocks
- Tests live next to code (`*_test.go`)

## Telegram bot commands

`/start`, `/help` -- welcome; `/new` -- reset session; `/stop` -- cancel active request; `/status` -- show request/session info.

## Environment filtering

Claude runner strips `CLAUDECODE`, `CLAUDE_CODE_*`, `CLAUDE_MANAGER_*`, `OTEL_*` env vars to prevent nested session interference.
