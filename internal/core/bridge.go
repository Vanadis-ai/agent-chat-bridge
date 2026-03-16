package core

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// FrontendRouting holds per-frontend config that Bridge uses to build BackendRequest.
type FrontendRouting struct {
	BackendName    string
	Model          string
	PermissionMode string
	SystemPrompt   string
	Agent          *AgentDef
}

// Bridge orchestrates message routing between frontends, backends, and plugins.
type Bridge struct {
	backends  map[string]LLMBackend
	plugins   []Plugin
	sessions  map[string]SessionStore
	routing   map[string]FrontendRouting

	mu             sync.RWMutex
	activeRequests map[string]context.CancelFunc
}

// NewBridge creates a bridge with the given routing, backends, plugins, and session stores.
func NewBridge(
	routing map[string]FrontendRouting,
	backends map[string]LLMBackend,
	plugins []Plugin,
	sessions map[string]SessionStore,
) *Bridge {
	return &Bridge{
		backends:       backends,
		plugins:        plugins,
		sessions:       sessions,
		routing:        routing,
		activeRequests: make(map[string]context.CancelFunc),
	}
}

// Handler returns a MessageHandler bound to a specific frontend.
func (b *Bridge) Handler(frontendName string) MessageHandler {
	route := b.routing[frontendName]

	return func(ctx context.Context, msg ChatMessage, deltaCh chan<- StreamDelta) (*Response, error) {
		defer close(deltaCh)

		key := frontendName + ":" + msg.ChatID + ":" + msg.UserID

		reqCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		if !b.setActive(key, cancel) {
			return nil, ErrRequestActive
		}
		defer b.clearActive(key)

		resp, err := b.runPlugins(reqCtx, msg, deltaCh)
		if err != nil || resp != nil {
			return resp, err
		}

		return b.runBackend(reqCtx, frontendName, route, msg, deltaCh)
	}
}

func (b *Bridge) runPlugins(
	ctx context.Context, msg ChatMessage, deltaCh chan<- StreamDelta,
) (*Response, error) {
	for _, p := range b.plugins {
		resp, err := p.HandleMessage(ctx, msg, deltaCh)
		if err != nil {
			return nil, err
		}
		if resp != nil {
			return resp, nil
		}
	}
	return nil, nil
}

func (b *Bridge) runBackend(
	ctx context.Context,
	frontendName string,
	route FrontendRouting,
	msg ChatMessage,
	deltaCh chan<- StreamDelta,
) (*Response, error) {
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

	backend, ok := b.backends[route.BackendName]
	if !ok {
		return nil, fmt.Errorf("backend %q not found", route.BackendName)
	}

	resp, err := backend.Run(ctx, req, deltaCh)
	if err != nil {
		return nil, err
	}

	if resp != nil && resp.SessionID != "" {
		store.Set(sessionKey, resp.SessionID)
	}

	return resp, nil
}

// CancelRequest cancels the active request for a user. Returns true if cancelled.
// CancelRequest cancels the active request context for a user.
// Does NOT remove from activeRequests -- the handler's deferred clearActive does that.
// This ensures no new request is accepted until the cancelled one fully unwinds.
func (b *Bridge) CancelRequest(frontendName, chatID, userID string) bool {
	key := frontendName + ":" + chatID + ":" + userID
	b.mu.Lock()
	defer b.mu.Unlock()
	cancel, exists := b.activeRequests[key]
	if !exists {
		return false
	}
	cancel()
	slog.Info("request cancelled", "key", key)
	return true
}

// ResetSession clears the session for a user.
func (b *Bridge) ResetSession(frontendName, chatID, userID string) {
	store, ok := b.sessions[frontendName]
	if !ok {
		return
	}
	store.Reset(chatID + ":" + userID)
}

// HasActive returns true if the user has an active request.
func (b *Bridge) HasActive(frontendName, chatID, userID string) bool {
	key := frontendName + ":" + chatID + ":" + userID
	b.mu.RLock()
	defer b.mu.RUnlock()
	_, exists := b.activeRequests[key]
	return exists
}

// SessionID returns the current session ID for a user.
func (b *Bridge) SessionID(frontendName, chatID, userID string) string {
	store, ok := b.sessions[frontendName]
	if !ok {
		return ""
	}
	return store.Get(chatID + ":" + userID)
}

func (b *Bridge) setActive(key string, cancel context.CancelFunc) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, exists := b.activeRequests[key]; exists {
		return false
	}
	b.activeRequests[key] = cancel
	return true
}

func (b *Bridge) clearActive(key string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.activeRequests, key)
}
