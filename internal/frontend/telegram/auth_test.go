package telegram

import (
	"testing"

	"github.com/vanadis-ai/agent-chat-bridge/internal/config"
)

func TestIsAuthorized(t *testing.T) {
	users := map[int64]*config.UserConfig{
		100: {WorkingDir: "/home/user1"},
		200: {WorkingDir: "/home/user2"},
	}

	tests := []struct {
		name       string
		userID     int64
		wantAuth   bool
		wantDir    string
	}{
		{"authorized user", 100, true, "/home/user1"},
		{"another authorized user", 200, true, "/home/user2"},
		{"unauthorized user", 999, false, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := IsAuthorized(users, tc.userID)
			if result.Authorized != tc.wantAuth {
				t.Errorf("Authorized = %v, want %v", result.Authorized, tc.wantAuth)
			}
			if tc.wantAuth && result.User.WorkingDir != tc.wantDir {
				t.Errorf("WorkingDir = %q, want %q", result.User.WorkingDir, tc.wantDir)
			}
		})
	}
}

func TestIsPrivateChat(t *testing.T) {
	tests := []struct {
		chatType string
		want     bool
	}{
		{"private", true},
		{"group", false},
		{"supergroup", false},
		{"channel", false},
	}
	for _, tc := range tests {
		t.Run(tc.chatType, func(t *testing.T) {
			if got := IsPrivateChat(tc.chatType); got != tc.want {
				t.Errorf("IsPrivateChat(%q) = %v, want %v", tc.chatType, got, tc.want)
			}
		})
	}
}
