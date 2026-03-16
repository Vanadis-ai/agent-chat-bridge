# Agent Chat Bridge Operations Specification

## Prerequisites

- Go 1.22+ installed
- Claude Code CLI v2.1+ installed and accessible at the path specified in config

---

# 1. Makefile

All operations go through `make`. The Makefile lives at the project root.

## Targets

### make build

Default target. Compiles the binary.

```makefile
build:
	go build -o agent-chat-bridge ./cmd/agent-chat-bridge
```

Output: `agent-chat-bridge` binary in the project root.

### make run

Builds and runs with default config (`configs/config.yaml`).

```makefile
run: build
	./agent-chat-bridge
```

### make test

Runs all unit tests.

```makefile
test:
	go test ./...
```

### make test-race

Runs all unit tests with Go race detector enabled.

```makefile
test-race:
	go test -race ./...
```

### make test-integration

Runs integration tests (build tag `integration`). These use mock Claude CLI and mock Telegram API.

```makefile
test-integration:
	go test -tags=integration ./internal/integration_test/...
```

### make test-all

Runs unit tests, race detection, and integration tests.

```makefile
test-all: test test-race test-integration
```

### make lint

Static analysis using `go vet`.

```makefile
lint:
	go vet ./...
```

### make clean

Removes build artifacts.

```makefile
clean:
	rm -f agent-chat-bridge
```

---

# 2. Running Locally

## Build

```sh
make build
```

## Run with default config

```sh
./agent-chat-bridge
```

Looks for `configs/config.yaml` in current directory.

## Run with custom config

```sh
./agent-chat-bridge --config /path/to/config.yaml
```

## Background run with log file

```sh
make start
```

Uses Makefile's `start` target (builds, runs in background, writes PID file).

## Follow logs

```sh
make logs
```

Logs are JSON (slog), `jq` makes them readable.

## Stop

```sh
make stop
```

Sends SIGTERM, triggers graceful shutdown (see spec.md section 3).

---

# 3. Configuration File

See `spec.md` section 1 for full schema.

Minimal working config:

```yaml
claude:
  binary: "/Users/alter/.local/bin/claude"

telegram_bots:
  assistant:
    token: "123456:ABC-DEF"
    users:
      123456789:
        working_dir: "/Users/alter/projects"
```

All optional fields will use defaults:
- `claude.timeout_minutes`: 10
- `telegram_bots.*.model`: Claude CLI default
- `telegram_bots.*.permission_mode`: bypassPermissions
- `telegram_bots.*.sessions`: `<bot_name>_sessions.json`
- `users.*.voice_dir`: `voice_inbox` (relative to working_dir)
- `users.*.files_dir`: `files_inbox` (relative to working_dir)

---

# 4. Health Check

No HTTP health endpoint (no web server). Health is determined by:

- **Process is running**: `pgrep agent-chat-bridge` returns PID
- **Logs are flowing**: recent entries in stderr/log file
- **Telegram polling active**: INFO log entry at startup with bot username for each bot

If the process crashes, it must be restarted manually (or by a process supervisor like launchd on macOS).

---

# 5. Troubleshooting

### Bot does not respond to messages

1. Check process is running: `pgrep agent-chat-bridge`
2. Check logs for errors: `make logs`
3. Verify bot token is correct (Telegram API auth failure logged at startup)
4. Verify user ID is in config (unauthorized attempts logged at WARN)
5. Verify bot is not in a group chat (private chat only)

### Claude requests timeout

1. Check `claude.timeout_minutes` in config (default: 10)
2. Check if Claude CLI is accessible: run `claude --version` manually
3. Check logs for SIGTERM/SIGKILL events

### Files not saving

1. Check `voice_dir`/`files_dir` paths exist and are writable
2. Check storage quota (10 GB limit per user per bot)
3. Check logs for "Failed to save file" errors

### "Previous request is still running" response

Per-user concurrency limit. Either:
- Wait for the running request to finish
- Use `/stop` to cancel it
