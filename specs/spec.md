# Telebridge Specification

## Purpose

Multi-bot Telegram-to-Claude Code bridge. A single process runs multiple Telegram bots, each acting as a separate agent with its own personality (system prompt), model, and user list. Each bot receives messages from authorized Telegram users in private chats, passes them to Claude Code CLI, and streams clean responses back. Handles file/voice/image attachments by saving them to per-bot-per-user directories and informing Claude about the file paths.

---

# 1. Configuration

## Requirement: Config File Loading

The system SHALL load configuration from a YAML file at startup. The default path SHALL be `config.yaml` in the working directory. An alternative path MAY be specified via `--config` CLI flag.

### Scenario: Successful config load
- **GIVEN** a valid `config.yaml` exists at the expected path
- **WHEN** the application starts
- **THEN** all configuration values are parsed and validated

### Scenario: Missing config file
- **GIVEN** no config file exists at the expected path
- **WHEN** the application starts
- **THEN** the application exits with a clear error message indicating the missing file path

### Scenario: Invalid config file
- **GIVEN** a config file exists but contains invalid YAML or missing required fields
- **WHEN** the application starts
- **THEN** the application exits with an error message specifying which field is invalid or missing

## Requirement: Config Structure

The configuration file SHALL contain the following sections:

```yaml
claude:
  binary: "/Users/alter/.local/bin/claude"  # REQUIRED - path to claude CLI
  timeout_minutes: 10                       # OPTIONAL - per-request timeout, default: 10
  max_concurrent: 5                         # OPTIONAL - global max concurrent Claude processes, default: 5

bots:
  obsidian:                                  # bot name (alphanumeric + underscore)
    token: "BOT_TOKEN_1"                     # REQUIRED - Telegram Bot API token
    model: "opus"                            # OPTIONAL - model alias or full name
    permission_mode: "bypassPermissions"     # OPTIONAL - default: bypassPermissions
    append_system_prompt: "You work with Obsidian vault..."  # OPTIONAL
    sessions: "obsidian_sessions.json"       # OPTIONAL - default: <bot_name>_sessions.json
    users:
      123456789:
        working_dir: "/path/to/vault"        # REQUIRED - CWD for claude process
        voice_dir: "voice_inbox"             # OPTIONAL - default: voice_inbox (relative to working_dir)
        files_dir: "files_inbox"             # OPTIONAL - default: files_inbox (relative to working_dir)
      987654321:
        working_dir: "/path/to/other/vault"

  translator:
    token: "BOT_TOKEN_2"
    model: "sonnet"
    permission_mode: "plan"
    append_system_prompt: "You are a translator. No tools needed."
    sessions: "translator_sessions.json"
    users:
      123456789:
        working_dir: "/path/to/translations"
```

The `bots` section defines one or more Telegram bots. Each bot has a unique name (used as identifier in logs and environment variable overrides), its own Telegram token, Claude CLI settings, and a `users` map. The `users` map within each bot serves a dual purpose: it defines the whitelist of authorized Telegram user IDs and configures per-user settings. Each user MUST have a `working_dir`. The `voice_dir` and `files_dir` are optional -- when omitted, they default to `voice_inbox` and `files_inbox` subdirectories inside the user's `working_dir`. Both relative (resolved from `working_dir`) and absolute paths are supported.

### Scenario: Environment variable override for token
- **GIVEN** the environment variable `TELEBRIDGE_OBSIDIAN_TOKEN` is set (pattern: `TELEBRIDGE_<BOT_NAME>_TOKEN`, uppercased)
- **WHEN** the application loads configuration
- **THEN** the environment variable value SHALL override the `bots.obsidian.token` from the YAML file

### Scenario: Claude binary validation
- **GIVEN** the configured `claude.binary` path
- **WHEN** the application starts
- **THEN** it SHALL verify the binary exists and is executable, exiting with an error if not

### Scenario: Per-user working directory validation
- **GIVEN** each user entry in each bot has a `working_dir` path
- **WHEN** the application starts
- **THEN** it SHALL verify every configured `working_dir` exists, exiting with an error indicating which bot and user has an invalid path (the bot MUST NOT create working directories automatically -- they are the user's project directories)

### Scenario: Per-user directory creation
- **GIVEN** configured `voice_dir` or `files_dir` directories (resolved per user) do not exist
- **WHEN** the application starts
- **THEN** it SHALL create the directories recursively for each user (equivalent to `mkdir -p`)

### Scenario: Relative path resolution
- **GIVEN** a user has `working_dir: "/home/user/project"` and `voice_dir: "voice_inbox"` (or default)
- **WHEN** the application resolves paths
- **THEN** the effective voice directory is `/home/user/project/voice_inbox`

### Scenario: Sessions file path validation
- **GIVEN** each bot's configured `sessions` path
- **WHEN** the application starts
- **THEN** it SHALL verify the parent directory exists and is writable for each bot, exiting with an error if not

### Scenario: Duplicate token detection
- **GIVEN** two bots are configured with the same Telegram token
- **WHEN** the application starts
- **THEN** it SHALL exit with an error indicating which bots share a token

### Scenario: Bot name validation
- **GIVEN** a bot name in the `bots` map
- **WHEN** the application loads configuration
- **THEN** bot names MUST match `[a-z][a-z0-9_]*` (lowercase alphanumeric + underscore, starting with a letter), exiting with an error if not

### Scenario: At least one bot required
- **GIVEN** the `bots` section is empty or missing
- **WHEN** the application starts
- **THEN** it SHALL exit with an error indicating at least one bot must be configured

### Scenario: At least one user per bot required
- **GIVEN** a bot has an empty `users` section
- **WHEN** the application starts
- **THEN** it SHALL exit with an error indicating the bot must have at least one user

---

# 2. Authentication

## Requirement: User Whitelist

Each bot SHALL only process messages from Telegram users whose numeric user ID is present as a key in that bot's `users` section. All other messages SHALL be rejected. A single Telegram user MAY be authorized in multiple bots.

### Scenario: Authorized user sends a message
- **GIVEN** user with Telegram ID 123456789 is a key in bot "obsidian"'s `users`
- **WHEN** they send a message to the "obsidian" bot
- **THEN** the message is processed normally

### Scenario: Unauthorized user sends a message
- **GIVEN** user with Telegram ID 999999999 is NOT a key in bot "obsidian"'s `users`
- **WHEN** they send a message to the "obsidian" bot
- **THEN** the bot responds with "Adios amigo" and does not process the message further (no Claude invocation)

### Scenario: Unauthorized user sends a command
- **GIVEN** user with Telegram ID 999999999 is NOT a key in this bot's `users`
- **WHEN** they send `/start` or any other command
- **THEN** the bot responds with "Adios amigo" and does not process the command

### Scenario: User authorized in one bot but not another
- **GIVEN** user 123456789 is in "obsidian"'s `users` but NOT in "translator"'s `users`
- **WHEN** they send a message to the "translator" bot
- **THEN** the "translator" bot responds with "Adios amigo"

### Scenario: Unauthorized access logging
- **GIVEN** an unauthorized user sends a message to any bot
- **WHEN** the bot rejects it
- **THEN** the bot logs bot_name, user_id, and message type (but NOT message content) at WARN level for audit purposes

## Requirement: Auth Check Position

The authorization check SHALL be the first operation performed on every incoming update, before any other processing.

### Scenario: Auth check order
- **GIVEN** an incoming Telegram update for a specific bot
- **WHEN** the bot processes it
- **THEN** the user ID check occurs before message type detection, file download, or Claude invocation

## Requirement: Private Chat Only

Each bot SHALL only respond to messages in private (direct) chats. Messages from group chats, supergroups, and channels SHALL be ignored. Note: in Telegram private chats, `chat_id` equals `user_id`. The specification uses `user_id` as the canonical identifier everywhere (config keys, session maps, log entries, file names).

### Scenario: Bot added to a group chat
- **GIVEN** any bot is added to a group chat
- **WHEN** messages are sent in the group
- **THEN** the bot ignores all messages (does not respond, does not invoke Claude)

### Scenario: Message from a channel
- **GIVEN** a message arrives from a channel where a bot is an admin
- **WHEN** the update arrives
- **THEN** the bot ignores it

---

# 3. Bot Lifecycle

## Requirement: Startup

The application SHALL start all configured bots on launch. Each bot runs its own long-polling loop in a separate goroutine. None SHALL use webhooks.

### Scenario: Successful startup
- **GIVEN** a valid configuration with correct bot tokens for all bots
- **WHEN** the application starts
- **THEN** it logs each bot's name and Telegram username, starts a polling goroutine for each, and logs readiness

### Scenario: Invalid bot token for one bot
- **GIVEN** bot "obsidian" has a valid token but bot "translator" has an invalid token
- **WHEN** the application starts
- **THEN** it exits with an error indicating which bot has an invalid token (all bots must be valid at startup)

## Requirement: Update Offset Tracking

Each bot relies on its own instance of the `go-telegram-bot-api` library for automatic update offset management. The library tracks the last processed update ID per bot and passes it to `getUpdates`, ensuring no update is processed twice.

## Requirement: Graceful Shutdown

The application SHALL handle SIGINT and SIGTERM signals for graceful shutdown of all bots.

### Scenario: SIGINT received during idle
- **GIVEN** all bots are running and no requests are active
- **WHEN** SIGINT is received
- **THEN** all bots stop polling and the process exits cleanly within 1 second

### Scenario: SIGINT received during active requests
- **GIVEN** multiple bots are running and Claude CLI processes are active for some users
- **WHEN** SIGINT is received
- **THEN** each active Claude process receives SIGTERM, the application waits up to 5 seconds for all to exit (escalating to SIGKILL if needed), sends "Request interrupted" to each affected Telegram chat, stops all polling, and exits

---

# 4. Bot Commands

## Requirement: Start Command

Each bot SHALL support the `/start` command as the initial greeting.

### Scenario: Authorized user sends /start
- **GIVEN** user is in this bot's `users`
- **WHEN** they send `/start`
- **THEN** the bot responds with: "Telebridge active. Send a message to start a conversation with Claude. Use /help for available commands."

### Scenario: Authorized user sends /start with existing session
- **GIVEN** user already has an active session with this bot
- **WHEN** they send `/start`
- **THEN** the bot responds with the same greeting (session is NOT reset)

## Requirement: Help Command

Each bot SHALL support the `/help` command to list available commands.

### Scenario: User sends /help
- **GIVEN** an authorized user
- **WHEN** they send `/help`
- **THEN** the bot responds with a list of commands and their descriptions:
  ```
  /new - Start a new conversation session
  /stop - Interrupt the current request
  /status - Show current session info
  /help - Show this help message
  ```

## Requirement: New Session Command

Each bot SHALL support the `/new` command to start a fresh conversation session.

### Scenario: User resets session
- **GIVEN** user has an active session with UUID "abc-123" in this bot
- **WHEN** they send `/new`
- **THEN** a new UUID is generated and associated with their chat_id in this bot's session store, and the bot responds with "New session started"

### Scenario: User has no previous session
- **GIVEN** user has never interacted with this bot before
- **WHEN** they send `/new`
- **THEN** a new UUID is generated and associated with their chat_id in this bot's session store, and the bot responds with "New session started"

### Scenario: User sends /new while request is active
- **GIVEN** a Claude CLI process is running for this user on this bot
- **WHEN** they send `/new`
- **THEN** the bot responds with "Cannot reset session while a request is running. Use /stop first."

## Requirement: Status Command

Each bot SHALL support the `/status` command to show current session info.

### Scenario: Status with active session
- **GIVEN** user has an active session with this bot
- **WHEN** they send `/status`
- **THEN** the bot responds with: session ID (first 8 chars), user's working directory, model (if configured), and whether a request is currently running

### Scenario: Status with no session
- **GIVEN** user has no active session with this bot
- **WHEN** they send `/status`
- **THEN** the bot responds with "No active session. Send a message to start one."

## Requirement: Stop Command

Each bot SHALL support the `/stop` command to interrupt a running Claude request.

### Scenario: Stop with active request
- **GIVEN** a Claude CLI process is running for this user on this bot
- **WHEN** they send `/stop`
- **THEN** the process receives SIGTERM (escalating to SIGKILL after 5 seconds if needed), the bot sends "Request stopped" to the chat

### Scenario: Stop with no active request
- **GIVEN** no Claude CLI process is running for this user on this bot
- **WHEN** they send `/stop`
- **THEN** the bot responds with "Nothing to stop"

## Requirement: Unknown Commands

Each bot SHALL ignore unknown commands without responding.

### Scenario: Unknown command received
- **GIVEN** user sends `/foo` or any unrecognized command
- **WHEN** the bot processes it
- **THEN** the bot does not respond and does not invoke Claude

---

# 5. Text Message Handling

## Requirement: Text to Claude

When an authorized user sends a text message, the bot SHALL pass it to Claude Code CLI and stream the response back.

### Scenario: Simple text message
- **GIVEN** user sends "What files are in the current directory?"
- **WHEN** the bot processes the message
- **THEN** it spawns `claude -p` with the message text and streams the response, using this bot's model/prompt/permissions and this user's working_dir

### Scenario: Message with special characters
- **GIVEN** user sends a message containing quotes, backticks, or newlines
- **WHEN** the bot passes it to Claude
- **THEN** the message is passed via stdin (not as a CLI argument) to avoid shell escaping issues

### Scenario: Empty message
- **GIVEN** user sends an empty message (e.g., only whitespace)
- **WHEN** the bot receives it
- **THEN** the bot ignores it without sending to Claude

### Scenario: Forwarded message
- **GIVEN** user forwards a message from another chat
- **WHEN** the bot processes it
- **THEN** the bot treats the forwarded text as a regular text message (forward metadata is ignored)

### Scenario: Reply to bot message
- **GIVEN** user replies to a previous bot message with new text
- **WHEN** the bot processes it
- **THEN** the bot treats the reply text as a regular new message (reply context is not passed to Claude -- session continuity is maintained via session_id)

### Scenario: Edited message
- **GIVEN** user edits a previously sent message
- **WHEN** the Telegram update contains an edited_message
- **THEN** the bot ignores it (edited messages are not reprocessed)

## Requirement: Concurrent Request Prevention

Each bot SHALL NOT allow more than one concurrent Claude request per user.

### Scenario: User sends message while request is active
- **GIVEN** a Claude CLI process is already running for this user on this bot
- **WHEN** they send another text message
- **THEN** the bot responds with "Previous request is still running. Use /stop to cancel it."

### Scenario: Same user active on different bots
- **GIVEN** user has an active request on bot "obsidian"
- **WHEN** they send a message to bot "translator"
- **THEN** the "translator" bot processes it independently (per-bot isolation)

---

# 6. File Handling

## Requirement: Voice Message Saving

When an authorized user sends a voice message, the bot SHALL download the audio file and save it to the user's voice directory for this bot.

### Scenario: Voice message received
- **GIVEN** user sends a voice message
- **WHEN** the bot processes it
- **THEN** the bot downloads the .oga file from Telegram API (Opus codec in OGG container), saves it as `<user_voice_dir>/<YYYYMMDD_HHMMSS>_<user_id>.oga`, and sends to Claude: "User sent a voice message. File saved at: <full_path>"

### Scenario: Voice message with caption
- **GIVEN** user sends a voice message with an attached caption text
- **WHEN** the bot processes it
- **THEN** the prompt to Claude includes both the caption and the file path: "User sent a voice message with caption: '<caption>'. File saved at: <full_path>"

### Scenario: Voice directory does not exist at runtime
- **GIVEN** the user's resolved voice directory was deleted after startup
- **WHEN** a voice message arrives
- **THEN** the bot recreates the directory before saving the file

## Requirement: Document Saving

When an authorized user sends a document (file), the bot SHALL download it and save to the user's files directory for this bot.

### Scenario: Document received
- **GIVEN** user sends a PDF document named "report.pdf"
- **WHEN** the bot processes it
- **THEN** the bot downloads the file, saves it as `<user_files_dir>/report.pdf`, and sends to Claude: "User sent a file. File saved at: <full_path>"

### Scenario: Document with caption
- **GIVEN** user sends a document with caption "Please review this"
- **WHEN** the bot processes it
- **THEN** the prompt to Claude includes: "User sent a file with caption: 'Please review this'. File saved at: <full_path>"

### Scenario: Filename collision
- **GIVEN** a file named "report.pdf" already exists in the user's files directory
- **WHEN** another file with the same name arrives
- **THEN** the new file is saved as `report_2.pdf` (incrementing suffix)

## Requirement: Photo Saving

When an authorized user sends a photo, the bot SHALL download the highest-resolution version and save it to the user's files directory.

### Scenario: Photo received
- **GIVEN** user sends a photo (Telegram provides multiple resolutions)
- **WHEN** the bot processes it
- **THEN** it downloads the largest available resolution, saves as `<user_files_dir>/<YYYYMMDD_HHMMSS>_<msg_id>_photo.jpg`, and sends to Claude: "User sent a photo. File saved at: <full_path>"

### Scenario: Photo with caption
- **GIVEN** user sends a photo with caption "What is in this image?"
- **WHEN** the bot processes it
- **THEN** the prompt to Claude includes: "User sent a photo with caption: 'What is in this image?'. File saved at: <full_path>"

## Requirement: Video Note (Round Video) Saving

When an authorized user sends a video note (round video message), the bot SHALL download it and save to the user's voice directory (treated as voice-like media).

### Scenario: Video note received
- **GIVEN** user sends a round video message
- **WHEN** the bot processes it
- **THEN** it downloads the .mp4 file, saves as `<user_voice_dir>/<YYYYMMDD_HHMMSS>_<user_id>.mp4`, and sends to Claude: "User sent a video note. File saved at: <full_path>"

## Requirement: Video Saving

When an authorized user sends a regular video, the bot SHALL download it and save to the user's files directory (subject to Telegram's 20MB download limit).

### Scenario: Video received
- **GIVEN** user sends a video
- **WHEN** the bot processes it
- **THEN** it downloads the file, saves as `<user_files_dir>/<YYYYMMDD_HHMMSS>_<msg_id>_video.mp4`, and sends to Claude: "User sent a video. File saved at: <full_path>"

### Scenario: Video with caption
- **GIVEN** user sends a video with caption
- **WHEN** the bot processes it
- **THEN** the prompt to Claude includes both the caption and the file path

## Requirement: Audio File Saving

When an authorized user sends an audio file (music, podcast), the bot SHALL download it and save to the user's files directory.

### Scenario: Audio received
- **GIVEN** user sends an audio file (e.g., song.mp3)
- **WHEN** the bot processes it
- **THEN** it downloads the file, saves using the original filename (with collision handling like documents), and sends to Claude: "User sent an audio file. File saved at: <full_path>"

### Scenario: Audio with caption
- **GIVEN** user sends an audio file with caption
- **WHEN** the bot processes it
- **THEN** the prompt to Claude includes both the caption and the file path

## Requirement: Ignored Message Types

Each bot SHALL silently ignore the following message types without invoking Claude:

- Stickers
- Animations (GIFs)
- Locations
- Contacts
- Polls
- Dice
- Venues
- Games

### Scenario: Ignored type received
- **GIVEN** user sends a sticker, animation, location, contact, poll, dice, venue, or game
- **WHEN** the bot processes it
- **THEN** the bot ignores it and does not invoke Claude

## Requirement: Filename Sanitization

The system SHALL sanitize all filenames before saving to disk to prevent path traversal and filesystem issues.

### Scenario: Filename with path traversal
- **GIVEN** user sends a document named `../../etc/passwd`
- **WHEN** the bot processes the filename
- **THEN** path separators and `..` components are stripped, resulting in a safe basename (e.g., `etc_passwd`)

### Scenario: Filename with special characters
- **GIVEN** user sends a document named `report (final) [v2].pdf`
- **WHEN** the bot processes the filename
- **THEN** the filename is preserved as-is (special characters other than path separators are allowed)

### Scenario: Dotfile filename
- **GIVEN** user sends a document named `.bashrc`
- **WHEN** the bot processes the filename
- **THEN** the leading dot is replaced with underscore: `_bashrc`

### Scenario: Filename exceeds 255 characters
- **GIVEN** user sends a document with a filename longer than 255 characters
- **WHEN** the bot processes the filename
- **THEN** the filename is truncated to 255 characters, preserving the file extension

### Scenario: Empty filename after sanitization
- **GIVEN** the filename after sanitization is empty (e.g., original was `../..`)
- **WHEN** the bot processes the filename
- **THEN** a generated name is used: `file_<YYYYMMDD_HHMMSS>`

## Requirement: Storage Quota

Each user within each bot SHALL have a storage quota of 10 GB across their `voice_dir` and `files_dir` combined.

### Scenario: File download within quota
- **GIVEN** user's storage usage is 5 GB
- **WHEN** they send a 10 MB file
- **THEN** the file is downloaded and saved normally

### Scenario: File download exceeds quota
- **GIVEN** user's storage usage is 9.95 GB
- **WHEN** they send a 100 MB file that would exceed 10 GB
- **THEN** the bot responds with "Storage quota exceeded (10 GB limit). Delete some files to free space."

### Scenario: Quota check implementation
- **GIVEN** a file download is requested
- **WHEN** the bot checks the quota
- **THEN** it calculates the total size of all files in the user's `voice_dir` and `files_dir` (non-recursive, top-level files only)

## Requirement: File Size Limit

Each bot SHALL respect Telegram Bot API file size limits.

### Scenario: File exceeds 20MB
- **GIVEN** user sends a file larger than 20MB (Telegram Bot API download limit)
- **WHEN** the bot attempts to download it
- **THEN** the bot responds with "File is too large to download (Telegram limit: 20MB)"

---

# 7. Claude Code CLI Integration

## Requirement: Process Spawning

Each bot SHALL spawn Claude Code CLI as a child process for each user request, using the bot's configured model/prompt/permissions and the user's working_dir.

### Scenario: Standard invocation
- **GIVEN** user sends a text message "Hello" to bot "obsidian"
- **WHEN** the bot invokes Claude
- **THEN** it executes:
  ```
  claude -p \
    --output-format stream-json \
    --session-id <session-uuid> \
    --verbose \
    --model <bot-model-if-configured> \
    --permission-mode <bot-permission-mode-or-default> \
    --append-system-prompt "<bot-system-prompt-if-configured>"
  ```
  with the user's message passed via stdin, and the user's `working_dir` (from `bots.<bot_name>.users.<user_id>.working_dir`) as the process CWD

### Scenario: Bot without optional flags
- **GIVEN** bot "translator" has no `model` configured
- **WHEN** the bot invokes Claude
- **THEN** the `--model` flag is omitted (Claude CLI uses its default)

## Requirement: Environment Isolation

The system SHALL ensure the spawned Claude process does not detect it is running inside another Claude session.

### Scenario: CLAUDECODE environment variable
- **GIVEN** the bot process itself was started from a Claude Code session (CLAUDECODE env var is set)
- **WHEN** spawning the Claude CLI subprocess
- **THEN** the CLAUDECODE environment variable SHALL be removed from the child process environment

## Requirement: Stdin Message Passing

The system SHALL pass user messages to Claude via stdin, NOT as command-line arguments.

### Scenario: Message with special characters
- **GIVEN** user sends: `Find files matching "*.md" and count them`
- **WHEN** the bot invokes Claude
- **THEN** the full message is written to the process stdin pipe and the pipe is closed, avoiding any shell escaping issues

## Requirement: Process Timeout

The system SHALL enforce a maximum runtime for Claude CLI processes. The timeout is configured via `claude.timeout_minutes` (default: 10), shared across all bots.

### Scenario: Process exceeds timeout
- **GIVEN** a Claude process has been running longer than the configured timeout
- **WHEN** the timeout is reached
- **THEN** the process receives SIGTERM, escalating to SIGKILL after 5 seconds if still running, and the bot sends "Request timed out after N minutes" to the chat

---

# 8. Stream-JSON Parsing

## Requirement: JSON Line Parsing

The system SHALL read stdout of the Claude CLI process line by line, parsing each line as a JSON object.

### Scenario: System init message
- **GIVEN** Claude outputs `{"type":"system","subtype":"init","session_id":"...",...}`
- **WHEN** the parser processes this line
- **THEN** it extracts and stores the `session_id` for future reference, and does NOT forward any text to Telegram

### Scenario: Assistant text message (complete)
- **GIVEN** Claude outputs `{"type":"assistant","message":{"content":[{"type":"text","text":"Hello!"}],"stop_reason":"end_turn"}}`
- **WHEN** the parser processes this line
- **THEN** it extracts "Hello!" from `message.content` where `type` is "text" and forwards it to the streaming formatter as a final update

### Scenario: Assistant partial message
- **GIVEN** Claude outputs `{"type":"assistant","message":{"content":[{"type":"text","text":"Hel"}]}}` (no `stop_reason` field)
- **WHEN** the parser processes this line
- **THEN** it extracts "Hel" and forwards it to the streaming formatter as an intermediate update. Each partial message contains the full accumulated text up to that point (not a delta).

### Scenario: Distinguishing partial from complete messages
- **GIVEN** the parser receives assistant messages
- **WHEN** deciding whether a message is partial or complete
- **THEN** it checks for the presence of `stop_reason` field in `message`: absent means partial, present (e.g., `"end_turn"`, `"tool_use"`) means complete

### Scenario: Assistant tool use message
- **GIVEN** Claude outputs a message with `content[].type` equal to "tool_use"
- **WHEN** the parser processes this line
- **THEN** it does NOT forward tool use content to Telegram (clean output requirement)

### Scenario: Result message
- **GIVEN** Claude outputs `{"type":"result","subtype":"success","result":"Final text",...}`
- **WHEN** the parser processes this line
- **THEN** it signals that the response is complete and provides the `result` text as the final output

### Scenario: Error result
- **GIVEN** Claude outputs `{"type":"result","subtype":"error","is_error":true,"result":"Error message",...}`
- **WHEN** the parser processes this line
- **THEN** it forwards "Error: <error message>" to Telegram

### Scenario: Rate limit event
- **GIVEN** Claude outputs `{"type":"rate_limit_event",...}`
- **WHEN** the parser processes this line
- **THEN** it is silently ignored (not forwarded to Telegram)

### Scenario: Malformed JSON line
- **GIVEN** Claude outputs a line that is not valid JSON
- **WHEN** the parser processes this line
- **THEN** it logs a warning and continues processing subsequent lines without crashing

### Scenario: Multiple text content blocks
- **GIVEN** Claude outputs a message with multiple content blocks of type "text"
- **WHEN** the parser processes this line
- **THEN** it concatenates all text blocks with a newline separator

---

# 9. Streaming Output to Telegram

## Requirement: Progressive Message Updates

The system SHALL display Claude's response progressively in Telegram by creating a placeholder message and updating it as text arrives.

### Scenario: Streaming begins
- **GIVEN** a Claude request starts
- **WHEN** the first text chunk arrives
- **THEN** the bot sends a new message with the initial text content

### Scenario: Subsequent text chunks
- **GIVEN** a message has been sent with partial text
- **WHEN** additional text chunks arrive
- **THEN** the bot updates the existing message using `editMessageText` with the accumulated text

### Scenario: Accumulated text exceeds 4096 during streaming
- **GIVEN** the bot is streaming and the accumulated text grows beyond 4096 characters
- **WHEN** the next edit would exceed the limit
- **THEN** the bot finalizes the current message (no more edits), sends a new message with the overflow text, and continues streaming into the new message. This may result in multiple messages during a single response.

### Scenario: Rate limit compliance
- **GIVEN** the Telegram API limits message edits
- **WHEN** text chunks arrive faster than one every 2 seconds
- **THEN** the bot buffers updates and sends at most 1 edit per 2 seconds, accumulating text between edits

### Scenario: Final update
- **GIVEN** the Claude process completes (result message received)
- **WHEN** the final text is available
- **THEN** the bot performs one final `editMessageText` with the complete response, regardless of rate limiting. The `result.result` field is the single source of truth for the final text -- it takes precedence over any previously streamed partial content.

### Scenario: User deletes bot message during streaming
- **GIVEN** the bot is streaming a response and the user deletes the bot's message
- **WHEN** the next `editMessageText` call fails with "message to edit not found"
- **THEN** the bot sends the remaining response as a new message

## Requirement: Long Message Splitting

The system SHALL split responses that exceed Telegram's message length limit.

### Scenario: Response exceeds 4096 characters
- **GIVEN** Claude's response is 6000 characters
- **WHEN** the final text is sent to Telegram
- **THEN** the bot splits it into multiple messages, breaking at paragraph boundaries when possible, never breaking inside code blocks

### Scenario: Code block preservation
- **GIVEN** Claude's response contains a code block that spans characters 3900-4200
- **WHEN** the bot splits the message
- **THEN** the split occurs BEFORE the code block starts (at character ~3900), not in the middle of it

### Scenario: Single code block exceeds 4096 characters
- **GIVEN** Claude's response contains a code block longer than 4096 characters
- **WHEN** the bot splits the message
- **THEN** the code block is split at line boundaries, each fragment wrapped in its own ``` block with the same language tag

## Requirement: Markdown Formatting

The system SHALL send messages using Telegram HTML parse mode.

Rationale: Telegram MarkdownV2 requires escaping 18+ special characters (`.`, `-`, `(`, `)`, `!`, `>`, `#`, `+`, `=`, `{`, `}`, `|`, `~`, etc.) outside of code blocks, making correct conversion from Claude's standard markdown fragile and error-prone. HTML parse mode only requires escaping `<`, `>`, `&` in non-tag content.

### Scenario: Claude returns markdown
- **GIVEN** Claude's response contains markdown formatting (bold, code blocks, lists, links)
- **WHEN** the bot sends it to Telegram
- **THEN** markdown is converted to Telegram HTML:
  - `**bold**` becomes `<b>bold</b>`
  - `_italic_` becomes `<i>italic</i>`
  - `` `inline code` `` becomes `<code>inline code</code>`
  - Fenced code blocks (```) become `<pre><code class="language-X">...</code></pre>` where X is the language tag (omit `class` if no language specified)
  - `[text](url)` becomes `<a href="url">text</a>`
  - Characters `<`, `>`, `&` in non-tag content are escaped as `&lt;`, `&gt;`, `&amp;`
  - Markdown lists and headings are passed as plain text (Telegram HTML does not support them natively)

### Scenario: HTML parsing failure
- **GIVEN** the formatted message causes a Telegram API "can't parse entities" error
- **WHEN** the send/edit fails
- **THEN** the bot retries sending the message as plain text (no parse mode) as a fallback

---

# 10. Session Management

## Requirement: Session Creation

Each bot SHALL automatically create a new session when a user sends their first message to that bot.

### Scenario: First message from user
- **GIVEN** user 123456789 has no existing session with bot "obsidian"
- **WHEN** they send their first message to "obsidian"
- **THEN** a new UUID v4 is generated and mapped to their chat_id in "obsidian"'s session store

### Scenario: Same user on different bots
- **GIVEN** user 123456789 has session "abc-123" with bot "obsidian"
- **WHEN** they send their first message to bot "translator"
- **THEN** a new independent UUID is generated for the "translator" bot (sessions are fully isolated between bots)

### Scenario: Returning user after restart
- **GIVEN** user had a session "abc-123" with bot "obsidian" before the process was restarted
- **WHEN** the bot starts and loads its persisted sessions
- **THEN** the user's session "abc-123" is restored and subsequent messages continue the conversation

## Requirement: Session Persistence

Each bot SHALL persist its session mappings to its own file on disk on every change (creation or reset). This ensures sessions survive both graceful shutdowns and crashes.

### Scenario: Session save on change
- **GIVEN** a new session is created or a session is reset via `/new` on bot "obsidian"
- **WHEN** the session map changes
- **THEN** the entire map is written atomically (write to temp file, then rename) to bot "obsidian"'s configured `sessions` file

### Scenario: Session file format
- **GIVEN** two users have active sessions with bot "obsidian"
- **WHEN** sessions are persisted
- **THEN** the file `obsidian_sessions.json` contains:
  ```json
  {
    "123456789": "uuid-1",
    "987654321": "uuid-2"
  }
  ```

### Scenario: Corrupted sessions file
- **GIVEN** a bot's sessions file exists but contains invalid JSON
- **WHEN** the application starts
- **THEN** it logs a warning (including bot name), starts with an empty session map for that bot, and overwrites the file on next save

### Scenario: Sessions file not writable
- **GIVEN** a bot's configured sessions file path is in a directory without write permissions
- **WHEN** the application starts
- **THEN** it exits with an error message indicating which bot's sessions file is not writable

## Requirement: Session Reset

The `/new` command SHALL create a new session for the current bot, discarding the previous session ID.

### Scenario: Session reset
- **GIVEN** user has session "old-uuid" with bot "obsidian"
- **WHEN** they send `/new` to "obsidian"
- **THEN** a new UUID is generated, replacing "old-uuid" in "obsidian"'s session store, and persisted to disk. Sessions with other bots are not affected.

---

# 11. Error Handling

## Requirement: Claude Process Errors

The system SHALL handle Claude CLI failures gracefully.

### Scenario: Claude binary not found
- **GIVEN** the configured `claude.binary` path does not exist
- **WHEN** any bot attempts to spawn the process
- **THEN** the bot sends "Claude CLI not found at configured path" to the user

### Scenario: Claude process exits with non-zero code
- **GIVEN** the Claude process exits with code 1
- **WHEN** the bot detects the exit
- **THEN** the bot sends "Claude exited with error. Check logs for details." to the user and logs stderr content with bot_name context

### Scenario: Claude process crashes mid-stream
- **GIVEN** the Claude process crashes while streaming output
- **WHEN** the bot detects the broken pipe / unexpected EOF
- **THEN** the bot appends "\n\n[Response interrupted]" to the current message

## Requirement: Telegram API Errors

The system SHALL handle Telegram API failures gracefully.

### Scenario: Message edit fails (message not modified)
- **GIVEN** the bot attempts to edit a message with the same content
- **WHEN** Telegram returns "message is not modified"
- **THEN** the bot silently ignores this error (it is expected during streaming)

### Scenario: Message edit fails (message too old)
- **GIVEN** the bot attempts to edit a message older than 48 hours
- **WHEN** Telegram returns an error
- **THEN** the bot sends a new message instead of editing

### Scenario: Message edit fails (message deleted by user)
- **GIVEN** the bot attempts to edit a message that was deleted by the user
- **WHEN** Telegram returns "message to edit not found"
- **THEN** the bot sends a new message instead of editing

### Scenario: Rate limit hit (429)
- **GIVEN** any bot receives a 429 Too Many Requests error
- **WHEN** processing the error
- **THEN** the bot waits for the `retry_after` duration before the next API call

## Requirement: File Download Errors

The system SHALL handle file download failures.

### Scenario: Telegram file download fails
- **GIVEN** the bot attempts to download a file from Telegram
- **WHEN** the download fails (network error, timeout)
- **THEN** the bot responds with "Failed to download file. Please try again."

### Scenario: Disk write fails
- **GIVEN** the bot downloads a file successfully
- **WHEN** writing to the user's configured directory fails (permissions, disk full)
- **THEN** the bot responds with "Failed to save file" and logs the system error with bot_name context

---

# 12. Logging

## Requirement: Structured Logging

The system SHALL log operational events to stderr using Go's `log/slog` package with JSON output format. Every log entry MUST include the `bot` field with the bot name for context.

## Requirement: Log Levels

The system SHALL use the following log levels:

- **DEBUG**: Detailed processing info (stream-json events, message edit timing). Disabled by default.
- **INFO**: Normal operations (startup, shutdown, request start/end, session creation/reset).
- **WARN**: Recoverable issues (malformed JSON line, corrupted sessions file, unauthorized access attempt).
- **ERROR**: Failures requiring attention (Claude crash, file save failure, Telegram API errors).

### Scenario: Request logging
- **GIVEN** a user sends a message to a bot
- **WHEN** the bot processes it
- **THEN** it logs at INFO level: timestamp, bot_name, user_id, message type (text/voice/file/command), and session_id

### Scenario: Error logging
- **GIVEN** an error occurs on a bot
- **WHEN** the error is handled
- **THEN** it logs at ERROR level: timestamp, bot_name, error message, context (user_id, operation)

### Scenario: No message content in logs
- **GIVEN** a user sends a text message
- **WHEN** the bot logs the request
- **THEN** the log does NOT contain the message text (privacy)

---

# 13. Concurrency

## Requirement: Global Concurrency Limit

The system SHALL enforce a global maximum number of concurrent Claude CLI processes across all bots, configured via `claude.max_concurrent` (default: 5). This prevents overloading the local machine.

### Scenario: Global limit reached
- **GIVEN** 5 Claude processes are already running (the configured max)
- **WHEN** a new user sends a message to any bot
- **THEN** the bot responds with "System is busy, try again later." and does NOT spawn a new process

### Scenario: Global limit implementation
- **GIVEN** the system needs to enforce the global limit
- **WHEN** a new request arrives
- **THEN** a shared semaphore (buffered channel of size `max_concurrent`) is used to gate process spawning

## Requirement: Per-Bot-Per-User Request Isolation

The system SHALL handle requests from multiple bots and users concurrently. Each bot limits each user to one active request, but a user MAY have concurrent active requests on different bots. Both the per-user limit and the global limit must be satisfied before spawning a process.

### Scenario: Two users send messages to the same bot simultaneously
- **GIVEN** user A and user B both send messages to bot "obsidian" at the same time
- **WHEN** the bot processes them
- **THEN** two separate Claude processes are spawned, each in its own goroutine, each streaming to its respective chat

### Scenario: Same user sends rapid messages to the same bot
- **GIVEN** user A sends "Hello" and immediately sends "World" to bot "obsidian"
- **WHEN** the bot processes the second message
- **THEN** the second message is rejected with "Previous request is still running. Use /stop to cancel it."

### Scenario: Same user active on two bots simultaneously
- **GIVEN** user A has an active request on bot "obsidian"
- **WHEN** user A sends a message to bot "translator"
- **THEN** bot "translator" processes it independently -- two Claude processes run concurrently for the same user on different bots

## Requirement: Safe Concurrent Access

The system SHALL protect shared state (per-bot session maps, per-bot active request tracking) with proper synchronization.

### Scenario: Concurrent session access within a bot
- **GIVEN** multiple goroutines access the same bot's session map
- **WHEN** one writes and another reads
- **THEN** access is synchronized via sync.RWMutex, preventing data races

### Scenario: Concurrent active request tracking within a bot
- **GIVEN** multiple goroutines check and modify a bot's active request map
- **WHEN** one user starts a request while another finishes
- **THEN** access to the active request map is synchronized via sync.Mutex, preventing data races

---

# 14. Graceful Degradation

## Requirement: Partial Response Delivery

The system SHALL deliver whatever text was generated even if the process fails mid-stream.

### Scenario: Claude crashes after producing partial output
- **GIVEN** Claude has streamed 500 characters of response
- **WHEN** the process crashes
- **THEN** the 500 characters already sent to Telegram remain visible, and "[Response interrupted]" is appended

## Requirement: Startup Recovery

The system SHALL recover gracefully from previous unclean shutdowns.

### Scenario: Stale sessions file
- **GIVEN** a bot's sessions file references session IDs from a previous run
- **WHEN** the bot starts
- **THEN** session IDs are loaded as-is (Claude Code maintains its own session history; stale UUIDs simply start fresh conversations on the Claude side)

