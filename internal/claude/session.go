package claude

import (
	"encoding/json"
	"log/slog"
	"os"
	"sync"
)

// SessionStore persists Claude session IDs per user for conversation continuity.
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[int64]string
	filePath string
}

// NewSessionStore creates a store and loads existing sessions from disk.
func NewSessionStore(filePath string) *SessionStore {
	s := &SessionStore{
		sessions: make(map[int64]string),
		filePath: filePath,
	}
	s.load()
	return s
}

// Get returns the session ID for a user, or empty string if none.
func (s *SessionStore) Get(userID int64) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessions[userID]
}

// Set stores a session ID for a user and persists to disk.
func (s *SessionStore) Set(userID int64, sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[userID] = sessionID
	s.save()
}

// Reset removes a user's session.
func (s *SessionStore) Reset(userID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, userID)
	s.save()
}

func (s *SessionStore) load() {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return
	}
	if err := json.Unmarshal(data, &s.sessions); err != nil {
		slog.Warn("corrupted session file, starting fresh", "path", s.filePath, "error", err)
		s.sessions = make(map[int64]string)
	}
}

func (s *SessionStore) save() {
	data, err := json.Marshal(s.sessions)
	if err != nil {
		slog.Error("failed to marshal sessions", "error", err)
		return
	}
	tmp := s.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		slog.Error("failed to write session file", "error", err)
		return
	}
	if err := os.Rename(tmp, s.filePath); err != nil {
		slog.Error("failed to rename session file", "error", err)
	}
}
