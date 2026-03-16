package core

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func tempSessionFile(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "sessions.json")
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
}

func TestFileSessionStoreGetSetReset(t *testing.T) {
	path := tempSessionFile(t)
	s := NewFileSessionStore(path)

	key := "100:200"
	if got := s.Get(key); got != "" {
		t.Errorf("Get empty store = %q, want empty", got)
	}

	s.Set(key, "sess-abc")
	if got := s.Get(key); got != "sess-abc" {
		t.Errorf("Get after Set = %q, want %q", got, "sess-abc")
	}

	s.Reset(key)
	if got := s.Get(key); got != "" {
		t.Errorf("Get after Reset = %q, want empty", got)
	}
}

func TestFileSessionStorePersistence(t *testing.T) {
	path := tempSessionFile(t)
	s1 := NewFileSessionStore(path)
	s1.Set("chat:user", "sess-123")
	s1.Set("chat2:user2", "sess-456")

	s2 := NewFileSessionStore(path)
	if got := s2.Get("chat:user"); got != "sess-123" {
		t.Errorf("reloaded Get = %q, want %q", got, "sess-123")
	}
	if got := s2.Get("chat2:user2"); got != "sess-456" {
		t.Errorf("reloaded Get = %q, want %q", got, "sess-456")
	}
}

func TestFileSessionStoreKeysWithColons(t *testing.T) {
	path := tempSessionFile(t)
	s := NewFileSessionStore(path)

	key := "frontend:123:456"
	s.Set(key, "sess-x")
	if got := s.Get(key); got != "sess-x" {
		t.Errorf("Get = %q, want %q", got, "sess-x")
	}

	s2 := NewFileSessionStore(path)
	if got := s2.Get(key); got != "sess-x" {
		t.Errorf("reloaded Get = %q, want %q", got, "sess-x")
	}
}

func TestFileSessionStoreConcurrency(t *testing.T) {
	path := tempSessionFile(t)
	s := NewFileSessionStore(path)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := "chat:user"
			s.Set(key, "sess")
			s.Get(key)
			s.Reset(key)
		}(i)
	}
	wg.Wait()
}

func TestFileSessionStoreLegacyMigration(t *testing.T) {
	path := tempSessionFile(t)
	writeFile(t, path, `{"123456789":"sess-1","987654321":"sess-2"}`)

	s := NewFileSessionStore(path)

	if got := s.Get("123456789"); got != "" {
		t.Errorf("bare key should not exist, got %q", got)
	}
	if got := s.Get("123456789:123456789"); got != "sess-1" {
		t.Errorf("migrated key = %q, want %q", got, "sess-1")
	}
	if got := s.Get("987654321:987654321"); got != "sess-2" {
		t.Errorf("migrated key = %q, want %q", got, "sess-2")
	}
}

func TestFileSessionStoreLegacyMigrationRoundTrip(t *testing.T) {
	path := tempSessionFile(t)
	writeFile(t, path, `{"123":"sess-old"}`)

	s := NewFileSessionStore(path)
	s.Set("111:222", "sess-new")

	s2 := NewFileSessionStore(path)
	if got := s2.Get("123:123"); got != "sess-old" {
		t.Errorf("migrated key after reload = %q, want %q", got, "sess-old")
	}
	if got := s2.Get("111:222"); got != "sess-new" {
		t.Errorf("new key after reload = %q, want %q", got, "sess-new")
	}
}

func TestFileSessionStoreCorruptedFile(t *testing.T) {
	path := tempSessionFile(t)
	writeFile(t, path, `{not valid json`)

	s := NewFileSessionStore(path)
	if got := s.Get("any"); got != "" {
		t.Errorf("corrupted file should start empty, got %q", got)
	}
}

func TestFileSessionStoreMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent", "sessions.json")
	s := NewFileSessionStore(path)
	if got := s.Get("any"); got != "" {
		t.Errorf("missing file should start empty, got %q", got)
	}
}

func TestMigrateLegacyKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		wantKeys []string
	}{
		{
			name:     "no migration needed",
			input:    map[string]string{"100:200": "a"},
			wantKeys: []string{"100:200"},
		},
		{
			name:     "bare keys migrated",
			input:    map[string]string{"123": "a", "456": "b"},
			wantKeys: []string{"123:123", "456:456"},
		},
		{
			name:     "mixed keys",
			input:    map[string]string{"123": "a", "100:200": "b"},
			wantKeys: []string{"123:123", "100:200"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := migrateLegacyKeys(tc.input)
			for _, k := range tc.wantKeys {
				if _, ok := result[k]; !ok {
					t.Errorf("expected key %q not found", k)
				}
			}
			if len(result) != len(tc.wantKeys) {
				t.Errorf("got %d keys, want %d", len(result), len(tc.wantKeys))
			}
		})
	}
}
