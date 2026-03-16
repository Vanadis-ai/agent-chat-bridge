package core

import "context"

// MessageHandler processes an incoming message and streams the response.
// Frontend creates deltaCh (buffered), passes it here, reads deltas for streaming UX.
// Bridge closes deltaCh when processing is complete.
type MessageHandler func(ctx context.Context, msg ChatMessage, deltaCh chan<- StreamDelta) (*Response, error)

// ChatFrontend receives messages from a chat platform and delivers responses.
type ChatFrontend interface {
	// Start begins listening for messages. Blocks until ctx is cancelled or
	// an unrecoverable error occurs. Returns nil on clean shutdown.
	Start(ctx context.Context, handler MessageHandler) error

	// Stop gracefully shuts down the frontend.
	Stop() error

	// Name returns the frontend instance name (e.g. "hugstein", "codebot").
	Name() string
}

// LLMBackend executes prompts against an LLM provider with streaming.
type LLMBackend interface {
	// Run sends a prompt to the LLM and streams text deltas to deltaCh.
	// MUST NOT close deltaCh -- the caller (Bridge) owns the close.
	// MUST finish all writes to deltaCh before returning.
	Run(ctx context.Context, req BackendRequest, deltaCh chan<- StreamDelta) (*Response, error)
}

// Plugin intercepts messages before they reach the backend.
type Plugin interface {
	// Init initializes the plugin with its config section.
	Init(cfg map[string]any) error

	// HandleMessage processes a message.
	// Returns non-nil response to short-circuit the pipeline (skip remaining plugins and backend).
	// Returns nil response to pass the message to the next plugin or backend.
	// MUST NOT close deltaCh -- the caller (Bridge) owns the close.
	// MUST finish all writes to deltaCh before returning.
	HandleMessage(ctx context.Context, msg ChatMessage, deltaCh chan<- StreamDelta) (*Response, error)
}

// SessionStore persists session IDs for conversation continuity.
// Keys use the format "chatID:userID".
type SessionStore interface {
	Get(key string) string
	Set(key, sessionID string)
	Reset(key string)
}

// BridgeCallbacks provides methods that frontends use to interact with Bridge.
// Avoids circular dependency (frontend does not import bridge).
type BridgeCallbacks interface {
	CancelRequest(frontendName, chatID, userID string) bool
	ResetSession(frontendName, chatID, userID string)
	HasActive(frontendName, chatID, userID string) bool
	SessionID(frontendName, chatID, userID string) string
}
