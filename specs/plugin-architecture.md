# Plugin Architecture Specification

## 1. Overview

Refactor agent-chat-bridge from a monolithic Telegram+Claude bridge into a modular architecture with pluggable frontends (chat platforms), backends (LLM providers), and plugins (message interceptors).

### Goals

- Any fork can add backends, frontends, and plugins without breaking upstream sync
- Current Telegram + Claude functionality preserved as default implementations
- Backward-compatible config format (old config works without changes)
- Backward-compatible session files (per-frontend files, same as current per-bot files)
- Thin core: only routes messages between frontend and backend through plugin pipeline
- Streaming, formatting, media handling stay in frontend/backend implementations

### Non-goals

- One frontend routing to multiple backends simultaneously (1 frontend : N backends)
- Plugin-to-plugin communication
- Hot-reload of plugins at runtime

**Clarification**: N frontends sharing 1 backend IS supported (N:1). Each frontend routes to exactly one backend, but multiple frontends may reference the same backend name.

---

## 2. Core Types

### 2.1 ChatMessage

Platform-agnostic message representation. Replaces direct use of `tgbotapi.Message`.

```go
package core

type ChatMessage struct {
    ID          string            // frontend-specific message ID
    ChatID      string            // conversation/chat identifier
    UserID      string            // sender identifier (string, not int64)
    Text        string            // full prompt text including embedded attachment paths
    WorkingDir  string            // per-user working directory
    Attachments []Attachment      // structural attachment info (for plugins)
    IsCommand   bool              // true if message is a bot command
    Command     string            // command name without slash (e.g. "new", "stop")
    Metadata    map[string]string // optional per-message key-value pairs
}
```

**Attachment contract**: Frontend populates BOTH fields:
- `Text` contains the full prompt with embedded file paths (e.g. `"[Photo saved to /path/photo.jpg]\nDescribe this"`) -- this is what the backend uses as the prompt. This preserves current behavior.
- `Attachments` contains structured attachment metadata -- for plugins that need to inspect, filter, or transform attachments programmatically.

`Metadata` is for optional per-message data only (e.g. frontend-specific debug info). It does NOT carry config (model, permission_mode, agent) -- that lives in Bridge's routing config (see Section 4).

### 2.2 Attachment

```go
type Attachment struct {
    Type     AttachmentType
    Path     string         // local file path after frontend downloads it
    Name     string         // original filename
    MimeType string
    Size     int64
    Duration int            // seconds, for audio/video/voice
}

type AttachmentType string

const (
    AttachmentImage    AttachmentType = "image"
    AttachmentAudio    AttachmentType = "audio"
    AttachmentVideo    AttachmentType = "video"
    AttachmentDocument AttachmentType = "document"
    AttachmentVoice    AttachmentType = "voice"
)
```

### 2.3 Response

Backend/plugin result passed back to frontend. Frontend decides how to render it.

```go
type Response struct {
    Text      string
    SessionID string
    IsError   bool
    Error     string
    CostUSD   float64
    NumTurns  int
}
```

### 2.4 StreamDelta

```go
type StreamDelta struct {
    Text string
}
```

### 2.5 BackendRequest

Built by Bridge from ChatMessage + routing config. Backend never sees ChatMessage directly.

```go
type BackendRequest struct {
    Prompt         string
    WorkingDir     string
    Model          string
    PermissionMode string
    SystemPrompt   string
    SessionID      string
    Agent          *AgentDef      // nil if no agent configured
}

type AgentDef struct {
    Name        string
    Description string
    Prompt      string
    Tools       []string // nil = default, empty = no tools, non-empty = only listed
}
```

**Rationale**: typed `AgentDef` instead of `map[string]any` avoids JSON-in-string hacks and gives compile-time safety. If a future backend needs different options, it can define its own request wrapper that embeds `BackendRequest`.

---

## 3. Interfaces

### 3.1 ChatFrontend

Receives messages from users and delivers responses. Owns polling/webhook loop, media download, streaming UX, message formatting.

```go
type ChatFrontend interface {
    // Start begins listening for messages. Blocks until ctx is cancelled or
    // an unrecoverable error occurs. Returns nil on clean shutdown.
    Start(ctx context.Context, handler MessageHandler) error

    // Stop gracefully shuts down the frontend.
    Stop() error

    // Name returns the frontend instance name (e.g. "hugstein", "codebot").
    Name() string
}
```

`MessageHandler` is a callback the core provides to the frontend:

```go
// MessageHandler processes an incoming message and streams the response.
// Frontend creates deltaCh (buffered), passes it here, reads deltas for streaming UX.
// Bridge closes deltaCh when processing is complete (whether handled by plugin or backend).
// The returned Response contains the final result.
type MessageHandler func(ctx context.Context, msg ChatMessage, deltaCh chan<- StreamDelta) (*Response, error)
```

**Frontend responsibilities:**
- Authentication (user whitelist check)
- Media download and saving (using `media` package)
- Converting platform message -> ChatMessage (including prompt building with attachment paths)
- Handling built-in commands (/new, /stop, /status, /help) via BridgeCallbacks
- Streaming UX: create deltaCh, read from it in a goroutine, render progressively
- Message formatting and splitting
- No concurrency control (Bridge owns it)
- No session state (Bridge owns it)

### 3.2 LLMBackend

Executes prompts against an LLM with streaming support.

```go
type LLMBackend interface {
    // Run sends a prompt to the LLM and streams text deltas to deltaCh.
    // MUST NOT close deltaCh -- the caller (Bridge) owns the close.
    // MUST finish all writes to deltaCh before returning.
    Run(ctx context.Context, req BackendRequest, deltaCh chan<- StreamDelta) (*Response, error)
}
```

**Backend responsibilities:**
- LLM communication (CLI subprocess, HTTP API, etc.)
- Streaming text chunks to deltaCh (write-only, never close)
- Environment filtering (for Claude: strip CLAUDECODE vars)
- Translating BackendRequest fields to SDK calls

### 3.3 Plugin

Intercepts messages before they reach the backend. Can transform, log, filter, or fully handle a message with streaming.

```go
type Plugin interface {
    // Init initializes the plugin with its config section.
    Init(cfg map[string]any) error

    // HandleMessage processes a message. Returns:
    //   response != nil  -> plugin handled it, skip backend
    //   response == nil   -> pass to next plugin or backend
    // deltaCh allows the plugin to stream its own response progressively.
    // MUST NOT close deltaCh -- the caller (Bridge) owns the close.
    // MUST finish all writes to deltaCh before returning.
    HandleMessage(ctx context.Context, msg ChatMessage, deltaCh chan<- StreamDelta) (*Response, error)
}
```

Plugin pipeline is ordered. First plugin that returns a non-nil response wins; remaining plugins and backend are skipped.

### 3.4 SessionStore

```go
type SessionStore interface {
    Get(key string) string
    Set(key, sessionID string)
    Reset(key string)
}
```

### 3.5 BridgeCallbacks

Interface that frontends use to interact with Bridge for commands. Avoids circular dependency (frontend does not import bridge).

```go
type BridgeCallbacks interface {
    CancelRequest(frontendName, chatID, userID string) bool
    ResetSession(frontendName, chatID, userID string)
    HasActive(frontendName, chatID, userID string) bool
    SessionID(frontendName, chatID, userID string) string
}
```

---

## 4. Core (Bridge)

The Bridge is the orchestrator. It wires frontends to backends through the plugin pipeline.

### 4.1 Routing config

Bridge holds per-frontend routing configuration. This is where model, permission_mode, system_prompt, and agent config live -- NOT in ChatMessage.Metadata.

```go
type FrontendRouting struct {
    BackendName    string
    Model          string
    PermissionMode string
    SystemPrompt   string
    Agent          *AgentDef
}

type Bridge struct {
    backends  map[string]LLMBackend
    plugins   []Plugin
    sessions  map[string]SessionStore      // per-frontend session stores
    routing   map[string]FrontendRouting   // frontend name -> routing config

    mu             sync.Mutex
    activeRequests map[string]context.CancelFunc // "frontend:chatID:userID" -> cancel
}
```

### 4.2 deltaCh ownership

**Bridge's Handler closure is the single owner of deltaCh lifetime.** Rule:

1. Frontend creates `deltaCh := make(chan StreamDelta, 100)` and passes to Handler
2. Handler runs `defer close(deltaCh)` immediately
3. Handler passes deltaCh to plugin or backend (write-only)
4. Plugin/backend write to deltaCh but NEVER close it
5. Plugin/backend MUST finish all writes to deltaCh before returning (no background goroutines writing after return -- that causes send-on-closed-channel panic)
6. When Handler returns, defer fires, deltaCh closes
7. Frontend's consumer goroutine reading from deltaCh sees close, drains remaining buffer, and finishes (guarded by WaitGroup)

This prevents: double-close panics, send-on-closed-channel panics, deadlocks from unclosed channels, lost tail of response.

### 4.3 Composite key format

All state keyed by `"frontendName:chatID:userID"`:
- Active requests: prevents concurrent processing for the same user in the same chat
- Sessions: isolates conversations per chat (e.g. Discord user in #general vs #dev has separate sessions)

For Telegram private chats, chatID == userID (as int64 strings), so behavior is identical to current code.

### 4.4 Per-frontend session stores

Bridge holds `map[string]SessionStore` -- one store per frontend, each with its own file path from config. This preserves current behavior where each bot has its own `<bot>_sessions.json` file.

```go
// In NewBridge:
sessions := map[string]SessionStore{}
for name, fcfg := range frontendConfigs {
    sessions[name] = NewFileSessionStore(fcfg.SessionsFile)
}
```

### 4.4.1 Session key migration

Current code stores session keys as bare int64 strings: `{"123456789": "session-uuid"}`.
New format uses `"chatID:userID"`: `{"123456789:123456789": "session-uuid"}`.

`FileSessionStore.load()` detects legacy keys (no `:` in key) and migrates them transparently:
- For each key without `:`, rewrite as `"key:key"` (correct for Telegram private chats where chatID == userID)
- Migrated map saved to disk on next `Set()` call
- No manual migration required; existing conversations continue seamlessly

### 4.5 Message flow

```
Frontend receives platform message
    |
    v
Frontend: auth check, download media, build prompt text, build ChatMessage
    |
    v
Frontend: create deltaCh, start goroutine reading deltaCh for streaming UX
    |
    v
Frontend calls handler(ctx, msg, deltaCh)
    |
    v
Bridge Handler:
    defer close(deltaCh)                          <-- single owner
    |
    v
    check/set active request (by frontend:chatID:userID)
    |
    v
    run plugin pipeline(ctx, msg, deltaCh)
    |--- plugin returns response -> return
    |--- all plugins pass -> continue
    |
    v
    look up FrontendRouting for this frontend
    |
    v
    build BackendRequest from msg + routing config + session store
    |
    v
    backend.Run(ctx, req, deltaCh)
    |
    v
    save session ID from response
    |
    v
    return response to frontend
    |
    v
Frontend: Streamer.Finalize(), render final response
```

### 4.6 Concurrency control

Bridge is the single owner of request state. Methods:

- `HasActive(frontendName, chatID, userID) bool`
- `CancelRequest(frontendName, chatID, userID) bool` -- cancels ctx, removes from map
- `ResetSession(frontendName, chatID, userID)` -- clears session in the frontend's store
- `SessionID(frontendName, chatID, userID) string` -- reads from the frontend's store

Frontend has NO activeRequests map. For /status, frontend calls `bridge.HasActive()` and `bridge.SessionID()`.

### 4.7 Frontend lifecycle error handling

main.go collects Start errors via error channel:

```go
errCh := make(chan error, len(frontends))
for _, fe := range frontends {
    h := bridge.Handler(fe.Name())
    go func(fe ChatFrontend) {
        if err := fe.Start(ctx, h); err != nil && ctx.Err() == nil {
            slog.Error("frontend failed", "name", fe.Name(), "error", err)
            errCh <- err
        }
    }(fe)
}

// Shutdown policy: if all frontends fail, cancel the process.
go func() {
    var failCount int
    for range errCh {
        failCount++
        if failCount >= len(frontends) {
            slog.Error("all frontends failed, shutting down")
            cancel()
        }
    }
}()
```

---

## 5. Configuration

### 5.1 New format

```yaml
backends:
  claude_main:
    type: claude
    binary: "/Users/alter/.local/bin/claude"
    timeout_minutes: 10

frontends:
  hugstein:
    type: telegram
    token: "BOT_TOKEN"
    backend: claude_main
    model: "opus"
    permission_mode: "bypassPermissions"
    append_system_prompt: "You are Hugstein..."
    sessions: "hugstein_sessions.json"
    users:
      123456789:
        working_dir: "/path/to/work"

  codebot:
    type: telegram
    token: "BOT_TOKEN_2"
    backend: claude_main
    model: "sonnet"
    agent:
      name: "coder"
      description: "Code assistant"
      prompt: "You write code."
      tools: ["Read", "Write", "Bash"]
    sessions: "codebot_sessions.json"
    users:
      123456789:
        working_dir: "/path/to/work"

plugins:
  - name: request_logger
    enabled: true
    config:
      log_file: "/tmp/requests.log"
```

### 5.2 Backward compatibility

If the config contains `claude` + `telegram_bots` (old format), the loader converts it to new format internally:

- `claude` section -> single backend named `default` with `type: claude`
- Each entry in `telegram_bots` -> frontend with `type: telegram`, `backend: default`
- `sessions` field preserved per-frontend (same filenames as before)
- No `plugins` section -> empty plugin list

Detection: if `backends` key exists, use new format; otherwise treat as legacy.

### 5.3 Config types

Typed structs for known backend/frontend types. Extensibility through Go interfaces, not generic maps.

```go
type Config struct {
    // New format
    Backends  map[string]BackendConfig  `yaml:"backends"`
    Frontends map[string]FrontendConfig `yaml:"frontends"`
    Plugins   []PluginConfig            `yaml:"plugins"`

    // Legacy fields (populated by YAML, converted by loader)
    Claude       *ClaudeConfig          `yaml:"claude,omitempty"`
    TelegramBots map[string]BotConfig   `yaml:"telegram_bots,omitempty"`
}

type BackendConfig struct {
    Type           string `yaml:"type"`           // "claude"
    Binary         string `yaml:"binary"`
    TimeoutMinutes int    `yaml:"timeout_minutes"`
}

type FrontendConfig struct {
    Type               string                `yaml:"type"`    // "telegram"
    Backend            string                `yaml:"backend"` // references backends map key
    Token              string                `yaml:"token"`
    Model              string                `yaml:"model"`
    PermissionMode     string                `yaml:"permission_mode"`
    AppendSystemPrompt string                `yaml:"append_system_prompt"`
    Agent              *AgentConfig          `yaml:"agent"`
    Sessions           string                `yaml:"sessions"`
    Users              map[int64]*UserConfig `yaml:"users"`
}

type PluginConfig struct {
    Name    string         `yaml:"name"`
    Enabled bool           `yaml:"enabled"`
    Config  map[string]any `yaml:"config"`
}
```

When a new backend type (e.g. "openai") is added, its fields are added to BackendConfig (Go YAML ignores unknown fields). Validation checks that required fields are present for the declared type.

---

## 6. Package Layout

```
cmd/agent-chat-bridge/main.go     -- creates Bridge, registers frontends/backends/plugins

internal/
  core/                            -- interfaces, types, Bridge orchestrator
    types.go                       -- ChatMessage, Attachment, Response, StreamDelta, BackendRequest, AgentDef
    interfaces.go                  -- ChatFrontend, LLMBackend, Plugin, SessionStore, BridgeCallbacks, MessageHandler
    bridge.go                      -- Bridge struct, Handler(), plugin pipeline, concurrency, session routing
    session.go                     -- FileSessionStore (implements SessionStore)

  backend/                         -- LLMBackend implementations
    claude/
      backend.go                   -- ClaudeBackend implements LLMBackend
      options.go                   -- buildOptions, env filtering (from claude/runner.go)

  frontend/                        -- ChatFrontend implementations
    telegram/
      frontend.go                  -- TelegramFrontend implements ChatFrontend
      handler.go                   -- message handling, auth, prompt building, ChatMessage conversion
      streamer.go                  -- progressive message editing
      downloader.go                -- media download via Telegram API
      commands.go                  -- /new, /stop, /status, /help (delegate to BridgeCallbacks)
      sender.go                    -- TelegramSender interface (for tests)

  config/                          -- config loading
    config.go                      -- new + legacy format, Load()
    defaults.go                    -- applyDefaults for both formats
    validate.go                    -- validation for both formats
    legacy.go                      -- old format detection + conversion

  formatter/                       -- unchanged, used by telegram frontend
  media/                           -- unchanged, used by frontends

  plugin/                          -- plugin implementations
    registry.go                    -- plugin name -> constructor, LoadPlugins()
    request_logger.go              -- example pass-through plugin
```

---

## 7. Proposals

Each proposal is an independent unit of work. They build on each other sequentially.
After each proposal the project compiles and all existing tests pass.

---

### Proposal 1: Core types and interfaces

**Goal**: define `internal/core/` package with all shared types and interfaces. No implementations.

**Files to create:**

`internal/core/types.go`:
- `AttachmentType` constants (image, audio, video, document, voice)
- `Attachment` struct (Type, Path, Name, MimeType, Size, Duration)
- `ChatMessage` struct (ID, ChatID, UserID, Text, WorkingDir, Attachments, IsCommand, Command, Metadata)
- `StreamDelta` struct (Text)
- `Response` struct (Text, SessionID, IsError, Error, CostUSD, NumTurns)
- `BackendRequest` struct (Prompt, WorkingDir, Model, PermissionMode, SystemPrompt, SessionID, Agent)
- `AgentDef` struct (Name, Description, Prompt, Tools)
- `ErrRequestActive` sentinel error

`internal/core/interfaces.go`:
- `MessageHandler` func type
- `ChatFrontend` interface (Start, Stop, Name)
- `LLMBackend` interface (Run)
- `Plugin` interface (Init, HandleMessage)
- `SessionStore` interface (Get, Set, Reset)
- `BridgeCallbacks` interface (CancelRequest, ResetSession, HasActive, SessionID)

**Tests** (`internal/core/types_test.go`):
- ChatMessage construction with attachments and WorkingDir
- AttachmentType constants have expected string values
- Response with error fields
- BackendRequest with AgentDef (not map)

**Exit criteria**: `go build ./...` passes, `go test ./internal/core/...` passes.

---

### Proposal 2: LLMBackend -- Claude implementation

**Goal**: wrap `claude/runner.go` into `ClaudeBackend` implementing `core.LLMBackend`. Extract session store to `core/FileSessionStore`.

**Files to create:**

`internal/backend/claude/backend.go`:
```go
type ClaudeBackend struct {
    binary         string
    timeoutMinutes int
}

func NewClaudeBackend(binary string, timeoutMinutes int) *ClaudeBackend

// Run implements core.LLMBackend.
// Translates core.BackendRequest -> SDK options.
// Writes deltas to deltaCh. Does NOT close deltaCh.
func (b *ClaudeBackend) Run(ctx context.Context, req core.BackendRequest, deltaCh chan<- core.StreamDelta) (*core.Response, error)
```

`internal/backend/claude/options.go`:
- `buildOptions(b *ClaudeBackend, req core.BackendRequest) []claudecode.Option`
- `buildAgentOptions(agent *core.AgentDef) []claudecode.Option` -- typed AgentDef, no map casting
- `filteredEnv() map[string]string` -- unchanged
- `mapPermissionMode(mode string) claudecode.PermissionMode` -- unchanged

`internal/core/session.go`:
```go
type FileSessionStore struct {
    mu       sync.RWMutex
    sessions map[string]string
    filePath string
}

func NewFileSessionStore(filePath string) *FileSessionStore
func (s *FileSessionStore) Get(key string) string
func (s *FileSessionStore) Set(key, sessionID string)
func (s *FileSessionStore) Reset(key string)
```

Key change from current code: keys are `string` not `int64`. File format: `{"chatID:userID": "session-uuid"}`.

**Legacy key migration in FileSessionStore.load()**:
- After loading JSON, scan all keys
- If a key does not contain `:`, it is a legacy int64 key
- Migrate: `"123456789"` -> `"123456789:123456789"` (for Telegram private chats, chatID == userID)
- Migrated map saved on next `Set()` call
- Existing conversations continue without manual intervention

**Files NOT deleted in this proposal**: `internal/claude/` stays untouched. Existing consumers (`main.go`, `bot/`) continue using it. New code (`backend/claude/`, `core/session.go`) is purely additive. Old `internal/claude/` is removed in Proposal 4 together with `internal/bot/` and main.go rewrite.

**Key difference from current code regarding deltaCh**: current `claude.Run()` calls `defer close(deltaCh)`. New `ClaudeBackend.Run()` does NOT close deltaCh -- Bridge owns that.

**Tests** (`internal/backend/claude/backend_test.go`):
- buildOptions produces correct SDK options
- buildAgentOptions from typed AgentDef (name, description, prompt, tools)
- Env filtering
- Permission mode mapping
- Run does NOT close deltaCh (verify channel remains open after Run returns)

**Tests** (`internal/core/session_test.go`):
- Get/Set/Reset with string keys
- Persistence to disk (write + reload)
- Concurrent access safety
- Keys with colons work correctly ("frontend:123:456")
- Legacy migration: load file with int64 keys, verify Get("123:123") returns the session
- Migration round-trip: load legacy, Set new key, reload, both old (migrated) and new keys present

**Exit criteria**: `go build ./...` passes, all tests pass. `internal/claude/` still exists and all its consumers still compile.

---

### Proposal 3: ChatFrontend -- Telegram implementation

**Goal**: wrap `bot/` into `telegram.Frontend` implementing `core.ChatFrontend`. Decouple from Claude -- use `core.MessageHandler` callback.

**Files to create:**

`internal/frontend/telegram/frontend.go`:
```go
type Frontend struct {
    name     string
    config   FrontendConfig
    sender   TelegramSender
    handler  core.MessageHandler
    bridge   core.BridgeCallbacks
    stopFunc context.CancelFunc
}

// No activeRequests map -- Bridge owns concurrency.

// FrontendConfig holds ONLY what the Telegram frontend itself needs.
// Model, PermissionMode, SystemPrompt, Agent, Sessions live in Bridge.FrontendRouting.
// This eliminates dual source of truth.
type FrontendConfig struct {
    Token string
    Users map[int64]*UserConfig
}

func NewFrontend(name string, cfg FrontendConfig, sender TelegramSender, bridge core.BridgeCallbacks) *Frontend
func (f *Frontend) Start(ctx context.Context, handler core.MessageHandler) error
func (f *Frontend) Stop() error
func (f *Frontend) Name() string
```

`internal/frontend/telegram/handler.go`:
- `handleUpdate(update)` -- auth, command routing, message handling
- `handleMessage(msg, userCfg)` -- builds ChatMessage, calls runRequest
- `buildChatMessage(msg *tgbotapi.Message, userCfg *UserConfig) core.ChatMessage`:
  - Downloads media first (if any)
  - Builds prompt text with embedded file paths (current behavior)
  - Populates both `Text` and `Attachments`
  - Sets `WorkingDir` from userCfg
  - Does NOT put model/permission_mode/agent in Metadata (Bridge has that)
- `runRequest(chatID int64, chatMsg core.ChatMessage)`:
  - Checks `f.bridge.HasActive(...)` -- if active, sends "use /stop" message
  - Creates `deltaCh := make(chan core.StreamDelta, 100)`
  - Creates Streamer, sends initial "..." message
  - Starts consumer goroutine with WaitGroup:
    ```go
    var wg sync.WaitGroup
    wg.Add(1)
    go func() {
        defer wg.Done()
        for delta := range deltaCh {
            streamer.Append(delta.Text)
        }
    }()
    ```
  - Calls `resp, err := f.handler(ctx, chatMsg, deltaCh)` (Bridge closes deltaCh on return)
  - `wg.Wait()` -- ensures all buffered deltas are drained before finalize
  - `streamer.Finalize()`
  - Error handling:
    - `err == core.ErrRequestActive` -> send "Previous request is still running. Use /stop to cancel it." (preserves current UX)
    - `err != nil` (other) -> send "Error: ..."
    - `resp.IsError` -> send "Claude error: ..."
    - success -> log cost/turns/session

`internal/frontend/telegram/commands.go`:
- `/new` -> `f.bridge.ResetSession(f.name, chatID, userID)`
- `/stop` -> `f.bridge.CancelRequest(f.name, chatID, userID)`
- `/status` -> `f.bridge.HasActive(...)` + `f.bridge.SessionID(...)`
- `/start`, `/help` -- unchanged

`internal/frontend/telegram/streamer.go` -- moved from bot/, unchanged.
`internal/frontend/telegram/downloader.go` -- moved from bot/, unchanged.
`internal/frontend/telegram/auth.go` -- moved from bot/, unchanged.
`internal/frontend/telegram/sender.go` -- moved from bot/ (TelegramSender interface).

**Files NOT deleted in this proposal**: `internal/bot/` and `internal/claude/` stay. `main.go` still imports them and the old wiring still works. New `frontend/telegram/` is created alongside `bot/` -- purely additive. Old packages deleted in Proposal 4 when main.go is rewritten.

**Tests** (`internal/frontend/telegram/*_test.go`):
- auth.go, sender.go tests: copied and adapted from bot/ package
- `buildChatMessage`: maps Telegram fields, populates Text with attachment paths AND Attachments structurally
- `runRequest`: calls MessageHandler, streams deltas to Streamer, handles close
- `runRequest`: WaitGroup ensures all buffered deltas consumed before Finalize
- Commands: /new calls ResetSession, /stop calls CancelRequest, /status calls HasActive+SessionID (mock BridgeCallbacks)
- Plugin-handled response with streaming: deltaCh receives deltas, then closes, consumer drains, Streamer finalizes correctly
- ErrRequestActive race: handler returns ErrRequestActive -> frontend sends "Previous request is still running" (not generic error)

**Exit criteria**: `go build ./...` passes (old main.go + old packages still compile; new frontend/telegram/ compiles and its tests pass).

---

### Proposal 4: Bridge orchestrator

**Goal**: implement `core.Bridge` that wires everything together. Rewrite `main.go`. Delete old `internal/bot/` and `internal/claude/`.

**Files to create:**

`internal/core/bridge.go`:
```go
type Bridge struct {
    backends  map[string]LLMBackend
    plugins   []Plugin
    sessions  map[string]SessionStore    // per-frontend
    routing   map[string]FrontendRouting

    mu             sync.Mutex
    activeRequests map[string]context.CancelFunc // "frontend:chatID:userID"
}

type FrontendRouting struct {
    BackendName    string
    Model          string
    PermissionMode string
    SystemPrompt   string
    Agent          *AgentDef
    SessionsFile   string
}

func NewBridge(
    routing map[string]FrontendRouting,
    backends map[string]LLMBackend,
    plugins []Plugin,
    sessions map[string]SessionStore,
) *Bridge
```

`Bridge.Handler(frontendName)` implementation:
```go
func (b *Bridge) Handler(frontendName string) MessageHandler {
    route := b.routing[frontendName]

    return func(ctx context.Context, msg ChatMessage, deltaCh chan<- StreamDelta) (*Response, error) {
        defer close(deltaCh) // Bridge owns deltaCh lifetime

        key := frontendName + ":" + msg.ChatID + ":" + msg.UserID

        // Concurrency check
        reqCtx, cancel := context.WithCancel(ctx)
        defer cancel()
        if !b.setActive(key, cancel) {
            return nil, ErrRequestActive
        }
        defer b.clearActive(key)

        // Plugin pipeline
        for _, p := range b.plugins {
            resp, err := p.HandleMessage(reqCtx, msg, deltaCh)
            if err != nil {
                return nil, err
            }
            if resp != nil {
                return resp, nil
            }
        }

        // Build BackendRequest from routing config + message
        store := b.sessions[frontendName]
        sessionKey := msg.ChatID + ":" + msg.UserID

        req := BackendRequest{
            Prompt:         msg.Text,
            WorkingDir:     msg.WorkingDir,
            Model:          route.Model,
            PermissionMode: route.PermissionMode,
            SystemPrompt:   route.SystemPrompt,
            SessionID:      store.Get(sessionKey),
            Agent:          route.Agent,
        }

        // Run backend
        resp, err := b.backends[route.BackendName].Run(reqCtx, req, deltaCh)
        if err != nil {
            return nil, err
        }

        // Update session
        if resp.SessionID != "" {
            store.Set(sessionKey, resp.SessionID)
        }

        return resp, nil
    }
}
```

Bridge implements `BridgeCallbacks`:
```go
func (b *Bridge) CancelRequest(frontendName, chatID, userID string) bool
func (b *Bridge) ResetSession(frontendName, chatID, userID string)
func (b *Bridge) HasActive(frontendName, chatID, userID string) bool
func (b *Bridge) SessionID(frontendName, chatID, userID string) string
```

**Rewrite `cmd/agent-chat-bridge/main.go`:**

Uses `run() error` pattern. All cleanup (pidfile removal) lives inside `run()` via defer, so it executes on both success and error returns. `main()` only does flag parsing and delegates to `run()`. `os.Exit(1)` in `main()` is safe because no defers are registered there.

```go
func main() {
    configPath := flag.String("config", defaultConfigPath(), "path to config file")
    pidFile := flag.String("pidfile", "", "write PID to this file")
    debug := flag.Bool("debug", false, "enable debug logging")
    flag.Parse()

    if *debug {
        slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
    }

    if err := run(*configPath, *pidFile); err != nil {
        slog.Error("fatal", "error", err)
        os.Exit(1)
    }
}

func run(configPath, pidFile string) error {
    // Pidfile setup with defer -- runs on any return from run()
    if pidFile != "" {
        if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
            return fmt.Errorf("write pidfile: %w", err)
        }
        defer os.Remove(pidFile)
    }

    // 1. Load config (legacy format)

    // 2. Create per-frontend session stores
    sessions := map[string]core.SessionStore{}
    for name, botCfg := range cfg.TelegramBots {
        sessions[name] = core.NewFileSessionStore(botCfg.Sessions)
    }

    // 3. Create backends
    backends := map[string]core.LLMBackend{
        "default": claude.NewClaudeBackend(cfg.Claude.Binary, cfg.Claude.TimeoutMinutes),
    }

    // 4. Build routing from legacy config
    routing := map[string]core.FrontendRouting{}
    for name, botCfg := range cfg.TelegramBots {
        routing[name] = core.FrontendRouting{
            BackendName:    "default",
            Model:          botCfg.Model,
            PermissionMode: botCfg.PermissionMode,
            SystemPrompt:   botCfg.AppendSystemPrompt,
            Agent:          mapAgent(botCfg.Agent),
            SessionsFile:   botCfg.Sessions,
        }
    }

    // 5. Create bridge
    bridge := core.NewBridge(routing, backends, nil, sessions)

    // 6. Create and start frontends
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    errCh := make(chan error, len(cfg.TelegramBots))

    var frontends []core.ChatFrontend
    for name, botCfg := range cfg.TelegramBots {
        api, err := tgbotapi.NewBotAPI(botCfg.Token)
        if err != nil {
            slog.Error("failed to create bot API", "bot", name, "error", err)
            continue
        }

        // FrontendConfig only has Token + Users; model/agent/sessions are in routing
        fe := telegram.NewFrontend(name, telegram.FrontendConfig{
            Token: botCfg.Token,
            Users: botCfg.Users,
        }, api, bridge)
        frontends = append(frontends, fe)

        h := bridge.Handler(name)
        go func(fe core.ChatFrontend, h core.MessageHandler) {
            if err := fe.Start(ctx, h); err != nil && ctx.Err() == nil {
                slog.Error("frontend failed", "name", fe.Name(), "error", err)
                errCh <- err
            }
        }(fe, h)
    }

    // 7. No frontends -> return error (defers still run, pidfile cleaned up)
    if len(frontends) == 0 {
        return fmt.Errorf("no frontends started")
    }

    // 8. Shutdown policy: if all running frontends fail, cancel the process
    go func() {
        var failCount int
        for range errCh {
            failCount++
            if failCount >= len(frontends) {
                slog.Error("all frontends failed, shutting down")
                cancel()
            }
        }
    }()

    // 9. Wait for signal or all-frontend-failure cancellation
    waitForShutdown(ctx, cancel, frontends)
    return nil
}

// waitForShutdown blocks until OS signal OR ctx cancellation (from errCh policy).
// Current main.go only listens to OS signals; this version also listens to ctx.Done().
func waitForShutdown(ctx context.Context, cancel context.CancelFunc, frontends []core.ChatFrontend) {
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

    select {
    case sig := <-sigCh:
        slog.Info("received signal, shutting down", "signal", sig)
    case <-ctx.Done():
        slog.Info("context cancelled, shutting down")
    }
    cancel()

    var wg sync.WaitGroup
    for _, fe := range frontends {
        wg.Add(1)
        go func(fe core.ChatFrontend) {
            defer wg.Done()
            if err := fe.Stop(); err != nil {
                slog.Error("frontend stop error", "name", fe.Name(), "error", err)
            }
        }(fe)
    }
    wg.Wait()
    slog.Info("shutdown complete")
}
```

**Files to delete**:
- `internal/bot/` -- all consumers now use `frontend/telegram/`
- `internal/claude/` -- all consumers now use `backend/claude/` + `core.FileSessionStore`

**Tests** (`internal/core/bridge_test.go`):
- Handler routes to correct backend based on frontend name
- BackendRequest built from FrontendRouting (model, permission_mode, system_prompt, agent), not from Metadata
- Plugin pipeline: first plugin returning response wins, remaining skipped
- Plugin pipeline: all pass -> backend runs
- Concurrency: second request for same frontend:chatID:userID returns ErrRequestActive
- Concurrency: same user in different chats can have concurrent requests
- CancelRequest cancels context of active request
- ResetSession clears session in the correct frontend's store
- SessionID read from correct frontend's store
- Session ID from Response saved to correct store with "chatID:userID" key
- deltaCh closed by Handler after backend returns
- deltaCh closed by Handler after plugin handles message
- Plugin-handled response: plugin streams to deltaCh, Handler closes deltaCh on return
- HasActive returns correct state before/during/after request

**Tests** (legacy session compat):
- Per-frontend session files: two frontends have separate session files
- Session keys in file: "chatID:userID" format

**Tests** (agent round-trip):
- Config with agent -> FrontendRouting.Agent populated -> BackendRequest.Agent populated -> backend receives typed AgentDef

**Exit criteria**: `go build ./...` passes, all tests pass, system works identically to before refactor.

---

### Proposal 5: Plugin system

**Goal**: plugin registry, config-driven loading, example plugin.

**Files to create:**

`internal/plugin/registry.go`:
```go
type Constructor func() core.Plugin

// Register adds a plugin constructor. Called from init().
func Register(name string, ctor Constructor)

// Create instantiates a plugin by name.
func Create(name string) (core.Plugin, error)

// LoadPlugins creates and initializes enabled plugins from config.
func LoadPlugins(configs []config.PluginConfig) ([]core.Plugin, error)
```

`internal/plugin/request_logger.go`:
```go
// RequestLogger logs every incoming message. Pass-through (always returns nil response).
type RequestLogger struct {
    logFile string
}

func init() {
    Register("request_logger", func() core.Plugin { return &RequestLogger{} })
}

func (p *RequestLogger) Init(cfg map[string]any) error
func (p *RequestLogger) HandleMessage(ctx context.Context, msg core.ChatMessage, deltaCh chan<- core.StreamDelta) (*core.Response, error) {
    // Log message info, return nil to pass through
}
```

**Files to modify:**
- `cmd/agent-chat-bridge/main.go` -- load plugins, pass to Bridge

**Tests** (`internal/plugin/registry_test.go`):
- Register + Create by name
- Create unknown name -> error
- LoadPlugins: initializes in order, skips disabled
- LoadPlugins: empty config -> empty slice

**Tests** (`internal/plugin/request_logger_test.go`):
- Init with config map
- HandleMessage returns nil (pass-through)
- HandleMessage does not close deltaCh

**Exit criteria**: `go build ./...` passes, all tests pass.

---

### Proposal 6: Config schema extension

**Goal**: new config format + legacy auto-detection and conversion.

**Files to create:**

`internal/config/legacy.go`:
```go
// isLegacyFormat detects old format (has claude + telegram_bots, no backends).
func isLegacyFormat(raw map[string]any) bool

// convertLegacy transforms Config legacy fields to new format:
//   claude              -> Backends["default"] type=claude
//   telegram_bots[name] -> Frontends[name] type=telegram, backend=default
//   sessions field      -> preserved per-frontend
//   (no plugins)        -> Plugins = []
func convertLegacy(cfg *Config)
```

**Files to modify:**

`internal/config/config.go`:
- Config struct gains Backends/Frontends/Plugins fields (keeping legacy fields for YAML unmarshalling)
- `Load()`: unmarshal raw map first, detect format, unmarshal to Config, convert if legacy, apply defaults, validate

`internal/config/validate.go`:
- `validateBackends()` -- type required, claude needs binary
- `validateFrontends()` -- type required, backend ref must exist in Backends map, Telegram type validates token/users/agent
- `validatePlugins()` -- name required, no duplicates

`internal/config/defaults.go`:
- Apply defaults for new format fields (same values as current)

**Update main.go** to use unified config:
```go
for name, bcfg := range cfg.Backends {
    switch bcfg.Type {
    case "claude":
        backends[name] = claude.NewClaudeBackend(bcfg.Binary, bcfg.TimeoutMinutes)
    }
}

for name, fcfg := range cfg.Frontends {
    routing[name] = core.FrontendRouting{
        BackendName:    fcfg.Backend,
        Model:          fcfg.Model,
        ...
    }
    switch fcfg.Type {
    case "telegram":
        fe := telegram.NewFrontend(name, ...)
        frontends = append(frontends, fe)
    }
}

plugins, err := plugin.LoadPlugins(cfg.Plugins)
if err != nil {
    return fmt.Errorf("failed to load plugins: %w", err)
}
```

**Update `configs/config.yaml.example`** with both formats documented.

**Tests** (`internal/config/legacy_test.go`):
- Old format detected as legacy
- New format detected as non-legacy
- convertLegacy: Backends["default"] has correct type/binary/timeout
- convertLegacy: each telegram_bot -> Frontend with correct backend ref
- convertLegacy: per-frontend sessions file preserved
- convertLegacy: Plugins = empty slice

**Tests** (`internal/config/config_test.go` -- extend):
- New format loads correctly
- Legacy format loads and auto-converts
- Validation: missing backend type, bad frontend backend ref, duplicate plugin names
- Validation: Telegram frontend requires token, users
- Defaults applied for both formats

**Exit criteria**: `go build ./...` passes, all tests pass, both config formats work.

---

## 8. Migration Strategy

- Proposals 1-4 are the core refactor. After Proposal 4, the system works identically to current but with new architecture.
- Proposal 5 adds plugin capability.
- Proposal 6 adds new config format (old format still works).
- Each proposal results in a working, testable system.
- No big-bang rewrite: each step preserves all existing functionality.
- Session files remain per-frontend with same filenames. Legacy int64 keys auto-migrated on first load.

---

## 9. Decisions

1. **Plugin streaming**: plugins receive `deltaCh` to stream their own responses. No transformation of backend stream. Bridge owns deltaCh close.
2. **Concurrency control**: Bridge is the single owner of active request state. Frontend has no activeRequests map.
3. **Session store**: interface with Get/Set/Reset. Per-frontend FileSessionStore instances (one file per frontend). Key format in store: `"chatID:userID"`. Legacy int64 keys migrated transparently.
4. **Composite key**: `"frontend:chatID:userID"` for Bridge-level state. Prevents cross-chat bleed for multi-channel platforms.
5. **Attachment contract**: Frontend populates both `Text` (with embedded paths, used as prompt) and `Attachments` (structural, for plugins).
6. **Config ownership**: Bridge holds per-frontend routing config (model, permission_mode, system_prompt, agent). `telegram.FrontendConfig` contains only Token + Users. No dual source of truth.
7. **Typed config**: BackendConfig/FrontendConfig have typed fields per known type. No generic `map[string]any` Options.
8. **Typed agent**: `AgentDef` struct in core, not `map[string]any`. Compile-time safety.
9. **N:1 mapping**: Multiple frontends can share one backend. One frontend maps to exactly one backend.
10. **Lifecycle errors**: Frontend.Start errors collected via errCh. Goroutine counts failures; if all frontends fail, process cancels.
11. **deltaCh drain safety**: Frontend uses `sync.WaitGroup` to wait for consumer goroutine to drain all buffered deltas before calling `Streamer.Finalize()`.
12. **Proposal independence**: Proposals 2 and 3 are purely additive (create new packages alongside old). `internal/bot/` and `internal/claude/` deleted in Proposal 4 together with main.go rewrite. Each proposal compiles and passes tests independently.
13. **waitForShutdown**: Listens to both OS signals and ctx.Done() (from all-frontends-failed cancellation). Current implementation only handles OS signals.
