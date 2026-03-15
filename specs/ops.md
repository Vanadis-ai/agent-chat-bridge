# Agent Chat Bridge Operations Specification

## Prerequisites

- Go 1.22+ installed
- Claude Code CLI v2.1+ installed and accessible at the path specified in config
- Docker (optional, for containerized deployment)

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

Builds and runs with default config (`config.yaml` in current directory).

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

### make docker-build

Builds Docker image.

```makefile
docker-build:
	docker build -t agent-chat-bridge .
```

### make docker-run

Runs Docker container with host network and required volume mounts.

```makefile
docker-run:
	docker run --rm \
		--network host \
		-v $(PWD)/config.yaml:/etc/agent-chat-bridge/config.yaml:ro \
		-v $(CLAUDE_BINARY):/usr/local/bin/claude:ro \
		-v $(HOME)/.claude:/root/.claude:ro \
		agent-chat-bridge --config /etc/agent-chat-bridge/config.yaml
```

Variables `CLAUDE_BINARY` defaults to the path from config. User overrides via `make docker-run CLAUDE_BINARY=/path/to/claude`.

---

# 2. Dockerfile

Multi-stage build. No docker-compose.

## Build stage

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /agent-chat-bridge ./cmd/agent-chat-bridge
```

- Alpine base for minimal build image
- `CGO_ENABLED=0` for static binary (no libc dependency)
- `GOOS=linux` for Linux target (even when building on macOS)

## Runtime stage

```dockerfile
FROM gcr.io/distroless/static-debian12
COPY --from=builder /agent-chat-bridge /agent-chat-bridge
ENTRYPOINT ["/agent-chat-bridge"]
```

- Distroless base: no shell, no package manager, minimal attack surface
- Only the compiled binary is present

## Volume mounts required at runtime

| Mount | Container path | Purpose |
|-------|---------------|---------|
| config.yaml | /etc/agent-chat-bridge/config.yaml | Application configuration |
| claude binary | /usr/local/bin/claude | Claude Code CLI executable |
| ~/.claude | /root/.claude | Claude CLI config, auth, session history |
| working directories | same paths as in config | User project directories (read/write) |
| voice/files directories | same paths as in config | Media storage directories (read/write) |

## Network

`--network host` is required because:
- Claude CLI may need to reach Anthropic API
- Telegram Bot API requires outbound HTTPS

## Example run

```sh
docker run --rm \
  --network host \
  -v /Users/alter/agent-chat-bridge/config.yaml:/etc/agent-chat-bridge/config.yaml:ro \
  -v /Users/alter/.local/bin/claude:/usr/local/bin/claude:ro \
  -v /Users/alter/.claude:/root/.claude:ro \
  -v /Users/alter/obsidian-vault:/Users/alter/obsidian-vault \
  -v /Users/alter/translations:/Users/alter/translations \
  agent-chat-bridge --config /etc/agent-chat-bridge/config.yaml
```

---

# 3. Running Locally (without Docker)

## Build

```sh
cd agent-chat-bridge
make build
```

## Run with default config

```sh
./agent-chat-bridge
```

Looks for `config.yaml` in current directory.

## Run with custom config

```sh
./agent-chat-bridge --config /path/to/config.yaml
```

## Run with environment variable token override

```sh
AGENT_CHAT_BRIDGE_OBSIDIAN_TOKEN=bot123:secret ./agent-chat-bridge
```

## Background run with log file

```sh
./agent-chat-bridge 2>agent-chat-bridge.log &
```

## Follow logs

```sh
tail -f agent-chat-bridge.log | jq .
```

Logs are JSON (slog), `jq` makes them readable.

## Stop

```sh
kill $(pgrep agent-chat-bridge)
```

Sends SIGTERM, triggers graceful shutdown (see spec.md section 3).

---

# 4. Running in Docker

## Build image

```sh
make docker-build
```

## Run container

```sh
make docker-run
```

Or manually with explicit mounts (see example in section 2).

## Stop container

```sh
docker stop <container_id>
```

Sends SIGTERM to the process inside the container, same graceful shutdown behavior.

## View logs

```sh
docker logs <container_id>
docker logs -f <container_id>
```

---

# 5. Configuration File

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
- `claude.max_concurrent`: 5
- `bots.*.model`: Claude CLI default
- `bots.*.permission_mode`: bypassPermissions
- `bots.*.sessions`: `<bot_name>_sessions.json`
- `users.*.voice_dir`: `voice_inbox` (relative to working_dir)
- `users.*.files_dir`: `files_inbox` (relative to working_dir)

---

# 6. Health Check

No HTTP health endpoint (no web server). Health is determined by:

- **Process is running**: `pgrep agent-chat-bridge` returns PID
- **Logs are flowing**: recent entries in stderr/log file
- **Telegram polling active**: INFO log entry at startup with bot username for each bot

If the process crashes, it must be restarted manually (or by a process supervisor like launchd on macOS).

---

# 7. Troubleshooting

### Bot does not respond to messages

1. Check process is running: `pgrep agent-chat-bridge`
2. Check logs for errors: `tail -20 agent-chat-bridge.log | jq .`
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

### "System is busy" response

Global concurrency limit reached (`claude.max_concurrent`). Either:
- Wait for running requests to finish
- Increase `max_concurrent` in config (requires restart)
- Use `/stop` on idle requests
