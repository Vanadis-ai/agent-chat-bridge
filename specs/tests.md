# Agent Chat Bridge Test Specification

## Conventions

- All tests use Go standard `testing` package
- Table-driven tests preferred
- Test files live next to the code they test (`*_test.go`)
- No external test frameworks
- Test helpers use `t.Helper()`

---

# 1. Config Loader Tests

Package: `internal/config`

### Test: Valid full config
- **GIVEN** a YAML file with all fields populated (claude, bots, users)
- **WHEN** `Load()` is called
- **THEN** all fields are parsed correctly, defaults are NOT applied for explicitly set values

### Test: Valid minimal config
- **GIVEN** a YAML file with only required fields (claude.binary, one bot with token and one user with working_dir)
- **WHEN** `Load()` is called
- **THEN** defaults are applied: timeout_minutes=10, max_concurrent=5, permission_mode=bypassPermissions, voice_dir=voice_inbox, files_dir=files_inbox, sessions=<bot_name>_sessions.json

### Test: Missing config file
- **GIVEN** a non-existent file path
- **WHEN** `Load()` is called
- **THEN** returns error containing the file path

### Test: Invalid YAML syntax
- **GIVEN** a file with broken YAML (unclosed quotes, bad indentation)
- **WHEN** `Load()` is called
- **THEN** returns error indicating YAML parse failure

### Test: Missing required claude.binary
- **GIVEN** a YAML with `claude` section but no `binary` field
- **WHEN** `Load()` is called
- **THEN** returns error mentioning `claude.binary`

### Test: Empty bots section
- **GIVEN** a YAML with `telegram_bots: {}`
- **WHEN** `Load()` is called
- **THEN** returns error indicating at least one bot is required

### Test: Bot without token
- **GIVEN** a bot entry missing the `token` field
- **WHEN** `Load()` is called
- **THEN** returns error mentioning the bot name and `token`

### Test: Bot without users
- **GIVEN** a bot entry with `users: {}`
- **WHEN** `Load()` is called
- **THEN** returns error indicating the bot must have at least one user

### Test: User without working_dir
- **GIVEN** a user entry missing `working_dir`
- **WHEN** `Load()` is called
- **THEN** returns error mentioning the bot name, user ID, and `working_dir`

### Test: Duplicate tokens across bots
- **GIVEN** two bots with the same `token` value
- **WHEN** `Load()` is called
- **THEN** returns error indicating which bots share the token

### Test: Invalid bot name
- **GIVEN** a bot named "My-Bot" (uppercase, hyphen)
- **WHEN** `Load()` is called
- **THEN** returns error indicating bot name must match `[a-z][a-z0-9_]*`

### Test: Environment variable override for token
- **GIVEN** a valid config and env var `AGENT_CHAT_BRIDGE_OBSIDIAN_TOKEN=override_token`
- **WHEN** `Load()` is called
- **THEN** bot "obsidian" has token "override_token"

### Test: Relative path resolution
- **GIVEN** user with `working_dir: "/home/user"` and `voice_dir: "inbox"`
- **WHEN** paths are resolved
- **THEN** effective voice_dir is `/home/user/inbox`

### Test: Absolute path preserved
- **GIVEN** user with `working_dir: "/home/user"` and `voice_dir: "/tmp/voice"`
- **WHEN** paths are resolved
- **THEN** effective voice_dir is `/tmp/voice`

### Test: Default sessions file
- **GIVEN** bot "obsidian" without explicit `sessions` field
- **WHEN** `Load()` is called
- **THEN** sessions path defaults to `obsidian_sessions.json`

---

# 2. Stream-JSON Parser Tests

Package: `internal/claude`

### Test: System init message
- **GIVEN** JSON line `{"type":"system","subtype":"init","session_id":"abc-123"}`
- **WHEN** parsed
- **THEN** returns event type=SystemInit with session_id="abc-123", no text output

### Test: Complete assistant message
- **GIVEN** JSON line with `type=assistant`, content with text "Hello!", and `stop_reason: "end_turn"`
- **WHEN** parsed
- **THEN** returns event type=AssistantText, text="Hello!", complete=true

### Test: Partial assistant message
- **GIVEN** JSON line with `type=assistant`, content with text "Hel", no `stop_reason`
- **WHEN** parsed
- **THEN** returns event type=AssistantText, text="Hel", complete=false

### Test: Tool use message filtered
- **GIVEN** JSON line with `type=assistant`, content containing `type: "tool_use"`
- **WHEN** parsed
- **THEN** returns event with no text (tool use is not forwarded)

### Test: Mixed content blocks (text + tool_use)
- **GIVEN** JSON line with content array: [{type:text, text:"Before"}, {type:tool_use, ...}]
- **WHEN** parsed
- **THEN** returns only the text "Before", tool_use is stripped

### Test: Multiple text content blocks
- **GIVEN** JSON line with content array: [{type:text, text:"A"}, {type:text, text:"B"}]
- **WHEN** parsed
- **THEN** returns text "A\nB" (joined with newline)

### Test: Success result
- **GIVEN** JSON line `{"type":"result","subtype":"success","result":"Final answer"}`
- **WHEN** parsed
- **THEN** returns event type=Result, text="Final answer", is_error=false

### Test: Error result
- **GIVEN** JSON line `{"type":"result","subtype":"error","is_error":true,"result":"Something broke"}`
- **WHEN** parsed
- **THEN** returns event type=Result, text="Something broke", is_error=true

### Test: Rate limit event ignored
- **GIVEN** JSON line `{"type":"rate_limit_event"}`
- **WHEN** parsed
- **THEN** returns event type=Ignored (no text, no error)

### Test: Malformed JSON
- **GIVEN** a line "not valid json {"
- **WHEN** parsed
- **THEN** returns error, parser continues (does not panic)

### Test: Empty line
- **GIVEN** an empty string
- **WHEN** parsed
- **THEN** skipped without error

### Test: Unknown type
- **GIVEN** JSON line `{"type":"unknown_future_type"}`
- **WHEN** parsed
- **THEN** returns event type=Ignored (forward-compatible)

---

# 3. Markdown-to-HTML Converter Tests

Package: `internal/formatter`

### Test: Bold
- **GIVEN** `**bold text**`
- **WHEN** converted
- **THEN** output is `<b>bold text</b>`

### Test: Italic
- **GIVEN** `_italic text_`
- **WHEN** converted
- **THEN** output is `<i>italic text</i>`

### Test: Inline code
- **GIVEN** `` `some code` ``
- **WHEN** converted
- **THEN** output is `<code>some code</code>`

### Test: Fenced code block with language
- **GIVEN** ````go\nfmt.Println("hi")\n````
- **WHEN** converted
- **THEN** output is `<pre><code class="language-go">fmt.Println("hi")\n</code></pre>`

### Test: Fenced code block without language
- **GIVEN** ````\nsome code\n````
- **WHEN** converted
- **THEN** output is `<pre><code>some code\n</code></pre>`

### Test: Link
- **GIVEN** `[text](https://example.com)`
- **WHEN** converted
- **THEN** output is `<a href="https://example.com">text</a>`

### Test: HTML special characters escaped
- **GIVEN** `Use <div> & "quotes"`
- **WHEN** converted
- **THEN** `<` and `>` and `&` are escaped as `&lt;`, `&gt;`, `&amp;` outside of tags

### Test: Special characters inside code block not double-escaped
- **GIVEN** a code block containing `if a < b && c > d`
- **WHEN** converted
- **THEN** characters inside `<code>` are escaped once: `if a &lt; b &amp;&amp; c &gt; d`

### Test: Plain text passthrough
- **GIVEN** `Just plain text with no formatting`
- **WHEN** converted
- **THEN** output equals input (no tags added)

### Test: Lists passed as plain text
- **GIVEN** `- item 1\n- item 2`
- **WHEN** converted
- **THEN** output preserves the dashes as plain text (no HTML list tags)

### Test: Headings passed as plain text
- **GIVEN** `## Section title`
- **WHEN** converted
- **THEN** output preserves `##` prefix as plain text

### Test: Nested formatting
- **GIVEN** `**bold and _italic_**`
- **WHEN** converted
- **THEN** output is `<b>bold and <i>italic</i></b>`

---

# 4. Message Splitter Tests

Package: `internal/formatter`

### Test: Short message not split
- **GIVEN** text of 500 characters
- **WHEN** split
- **THEN** returns single chunk

### Test: Exact 4096 not split
- **GIVEN** text of exactly 4096 characters
- **WHEN** split
- **THEN** returns single chunk

### Test: Split at paragraph boundary
- **GIVEN** text of 5000 characters with a paragraph break (`\n\n`) at position 3800
- **WHEN** split
- **THEN** first chunk ends at position 3800, second chunk contains the rest

### Test: Split preserves code block
- **GIVEN** text where a code block starts at position 3900 and ends at 4200
- **WHEN** split
- **THEN** split occurs before the code block (not inside it)

### Test: Oversized code block split at line boundaries
- **GIVEN** a single code block of 5000 characters
- **WHEN** split
- **THEN** two chunks, each wrapped in its own ``` block with the same language tag

### Test: Multiple splits needed
- **GIVEN** text of 15000 characters
- **WHEN** split
- **THEN** returns 4 chunks, each <= 4096 characters

### Test: No good break point
- **GIVEN** text of 5000 characters with no paragraph breaks, no code blocks
- **WHEN** split
- **THEN** splits at last newline before 4096, or at 4096 if no newlines

---

# 5. Filename Sanitizer Tests

Package: `internal/media`

### Test: Normal filename preserved
- **GIVEN** `report.pdf`
- **WHEN** sanitized
- **THEN** returns `report.pdf`

### Test: Path traversal stripped
- **GIVEN** `../../etc/passwd`
- **WHEN** sanitized
- **THEN** returns `etc_passwd` (separators and `..` replaced)

### Test: Dotfile prefix replaced
- **GIVEN** `.bashrc`
- **WHEN** sanitized
- **THEN** returns `_bashrc`

### Test: Long filename truncated
- **GIVEN** filename of 300 characters with extension `.pdf`
- **WHEN** sanitized
- **THEN** returns filename of 255 characters, ending with `.pdf`

### Test: Empty after sanitization
- **GIVEN** `../..`
- **WHEN** sanitized
- **THEN** returns generated name like `file_20260308_143022`

### Test: Special characters preserved
- **GIVEN** `report (final) [v2].pdf`
- **WHEN** sanitized
- **THEN** returns `report (final) [v2].pdf`

### Test: Windows path separators
- **GIVEN** `folder\subfolder\file.txt`
- **WHEN** sanitized
- **THEN** returns `folder_subfolder_file.txt`

---

# 6. Filename Collision Resolver Tests

Package: `internal/media`

### Test: No collision
- **GIVEN** target path `report.pdf` does not exist on disk
- **WHEN** resolved
- **THEN** returns `report.pdf`

### Test: Single collision
- **GIVEN** `report.pdf` exists, `report_2.pdf` does not
- **WHEN** resolved
- **THEN** returns `report_2.pdf`

### Test: Multiple collisions
- **GIVEN** `report.pdf`, `report_2.pdf`, `report_3.pdf` all exist
- **WHEN** resolved
- **THEN** returns `report_4.pdf`

### Test: File without extension
- **GIVEN** `README` exists
- **WHEN** resolved
- **THEN** returns `README_2`

---

# 7. Storage Quota Tests

Package: `internal/media`

### Test: Within quota
- **GIVEN** user's voice_dir + files_dir total 5 GB
- **WHEN** checking if 100 MB file fits
- **THEN** returns ok

### Test: Exceeds quota
- **GIVEN** user's voice_dir + files_dir total 9.95 GB
- **WHEN** checking if 100 MB file fits
- **THEN** returns quota exceeded error

### Test: Empty directories
- **GIVEN** user's voice_dir and files_dir are empty
- **WHEN** checking usage
- **THEN** returns 0 bytes used

### Test: Only counts top-level files
- **GIVEN** files_dir contains files and a subdirectory with files inside
- **WHEN** calculating usage
- **THEN** only top-level file sizes are counted (subdirectories ignored)

---

# 8. Auth Tests

Package: `internal/bot`

### Test: Authorized user
- **GIVEN** bot config with user 123456789 in users map
- **WHEN** checking auth for user 123456789
- **THEN** returns authorized, provides user config

### Test: Unauthorized user
- **GIVEN** bot config with user 123456789 in users map
- **WHEN** checking auth for user 999999999
- **THEN** returns unauthorized

### Test: User in one bot but not another
- **GIVEN** two bot configs, user 123456789 only in first
- **WHEN** checking auth against second bot
- **THEN** returns unauthorized

### Test: Private chat only
- **GIVEN** an update from a group chat (chat.type = "group")
- **WHEN** checked
- **THEN** rejected regardless of user ID

---

# 9. Integration Tests

Package: `internal/integration_test` (build tag: `integration`)

These tests verify the full pipeline using mocks.

### Mock Claude CLI

A test binary (`testdata/mock_claude`) that:
- Reads stdin
- Outputs predefined stream-json lines to stdout based on input
- Supports scenarios: normal response, partial messages, error result, timeout (sleep), crash (exit 1)

### Mock Telegram API

An `httptest.Server` that:
- Accepts `getMe`, `getUpdates`, `sendMessage`, `editMessageText`
- Records all API calls for assertion
- Simulates injected updates via `getUpdates` responses

### Test: Full text message flow
- **GIVEN** mock claude outputs system init + 3 partial messages + result
- **WHEN** a text update is injected
- **THEN** mock Telegram receives: 1 sendMessage + N editMessageText calls, final text matches result

### Test: Voice message flow
- **GIVEN** mock Telegram serves a downloadable .oga file
- **WHEN** a voice update is injected
- **THEN** file is saved to user's voice_dir, claude receives prompt with file path

### Test: Concurrent request rejection
- **GIVEN** a slow mock claude (takes 5 seconds)
- **WHEN** two text updates arrive in rapid succession for the same user
- **THEN** first spawns claude, second gets "Previous request is still running" reply

### Test: /stop interrupts process
- **GIVEN** a slow mock claude running
- **WHEN** /stop update arrives
- **THEN** claude process is killed, "Request stopped" sent to chat

### Test: Multi-bot isolation
- **GIVEN** two bots configured, same user in both
- **WHEN** user sends message to both bots simultaneously
- **THEN** each bot spawns its own claude process with its own working_dir and session
