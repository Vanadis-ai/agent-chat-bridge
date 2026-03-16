package core

import "errors"

// ErrRequestActive is returned by Bridge when a user already has an active request.
// Frontends should check errors.Is(err, ErrRequestActive) and show a user-friendly message.
var ErrRequestActive = errors.New("request already active for this user")

// AttachmentType identifies the kind of media attachment.
type AttachmentType string

const (
	AttachmentImage    AttachmentType = "image"
	AttachmentAudio    AttachmentType = "audio"
	AttachmentVideo    AttachmentType = "video"
	AttachmentDocument AttachmentType = "document"
	AttachmentVoice    AttachmentType = "voice"
)

// Attachment holds metadata about a downloaded file attached to a message.
type Attachment struct {
	Type     AttachmentType
	Path     string // local file path after frontend downloads it
	Name     string // original filename
	MimeType string
	Size     int64
	Duration int // seconds, for audio/video/voice
}

// ChatMessage is a platform-agnostic message representation.
// Frontend populates both Text (full prompt with embedded file paths) and
// Attachments (structural metadata for plugins).
type ChatMessage struct {
	ID          string
	ChatID      string
	UserID      string
	Text        string // full prompt text including embedded attachment paths
	WorkingDir  string // per-user working directory
	Attachments []Attachment
	IsCommand   bool
	Command     string            // command name without slash (e.g. "new", "stop")
	Metadata    map[string]string // optional per-message key-value pairs
}

// StreamDelta is a chunk of text emitted during streaming.
type StreamDelta struct {
	Text string
}

// Response is the result of a backend or plugin invocation.
type Response struct {
	Text      string
	SessionID string
	IsError   bool
	Error     string
	CostUSD   float64
	NumTurns  int
}

// AgentDef defines a custom agent for the backend.
// Tools semantics: nil = default tools, empty slice = no tools, non-empty = only listed tools.
type AgentDef struct {
	Name        string
	Description string
	Prompt      string
	Tools       []string
}

// BackendRequest is built by Bridge from ChatMessage + routing config.
type BackendRequest struct {
	Prompt         string
	WorkingDir     string
	Model          string
	PermissionMode string
	SystemPrompt   string
	SessionID      string
	Agent          *AgentDef // nil if no agent configured
}
