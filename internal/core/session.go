package core

import (
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"sync"
)

// FileSessionStore implements SessionStore using a JSON file on disk.
// Keys use the format "chatID:userID". Legacy files with bare int64 keys
// (e.g. "123456789") are migrated to "123456789:123456789" on load.
type FileSessionStore struct {
	mu       sync.RWMutex
	sessions map[string]string
	filePath string
}

// NewFileSessionStore creates a store and loads existing sessions from disk.
func NewFileSessionStore(filePath string) *FileSessionStore {
	s := &FileSessionStore{
		sessions: make(map[string]string),
		filePath: filePath,
	}
	s.load()
	return s
}

// Get returns the session ID for a key, or empty string if none.
func (s *FileSessionStore) Get(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessions[key]
}

// Set stores a session ID and persists to disk.
// Skips write if value is unchanged (dirty check).
func (s *FileSessionStore) Set(key, sessionID string) {
	s.mu.Lock()
	if s.sessions[key] == sessionID {
		s.mu.Unlock()
		return
	}
	s.sessions[key] = sessionID
	snapshot := s.snapshotLocked()
	s.mu.Unlock()
	s.persist(snapshot)
}

// Reset removes a key and persists to disk.
func (s *FileSessionStore) Reset(key string) {
	s.mu.Lock()
	delete(s.sessions, key)
	snapshot := s.snapshotLocked()
	s.mu.Unlock()
	s.persist(snapshot)
}

func (s *FileSessionStore) load() {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return
	}
	var raw map[string]string
	if err := json.Unmarshal(data, &raw); err != nil {
		slog.Warn("corrupted session file, starting fresh",
			"path", s.filePath, "error", err)
		return
	}
	s.sessions = migrateLegacyKeys(raw)
}

// snapshotLocked returns a copy of sessions. Caller must hold mu.
func (s *FileSessionStore) snapshotLocked() map[string]string {
	cp := make(map[string]string, len(s.sessions))
	for k, v := range s.sessions {
		cp[k] = v
	}
	return cp
}

// persist writes the snapshot to disk. Does not hold mu during I/O.
func (s *FileSessionStore) persist(snapshot map[string]string) {
	data, err := json.Marshal(snapshot)
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

// migrateLegacyKeys converts bare int64 keys ("123") to composite keys ("123:123").
// For Telegram private chats, chatID == userID, so this is correct.
func migrateLegacyKeys(raw map[string]string) map[string]string {
	result := make(map[string]string, len(raw))
	for k, v := range raw {
		if strings.Contains(k, ":") {
			result[k] = v
		} else {
			result[k+":"+k] = v
		}
	}
	return result
}
