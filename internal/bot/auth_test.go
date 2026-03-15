package bot

import (
	"testing"

	"github.com/vanadis-ai/agent-chat-bridge/internal/config"
)

func TestAuthAuthorizedUser(t *testing.T) {
	users := map[int64]*config.UserConfig{
		123456789: {WorkingDir: "/home/user"},
	}
	result := IsAuthorized(users, 123456789)
	if !result.Authorized {
		t.Error("expected authorized")
	}
	if result.User == nil {
		t.Fatal("expected user config")
	}
	if result.User.WorkingDir != "/home/user" {
		t.Errorf("working_dir = %q", result.User.WorkingDir)
	}
}

func TestAuthUnauthorizedUser(t *testing.T) {
	users := map[int64]*config.UserConfig{
		123456789: {WorkingDir: "/home/user"},
	}
	result := IsAuthorized(users, 999999999)
	if result.Authorized {
		t.Error("expected unauthorized")
	}
	if result.User != nil {
		t.Error("expected nil user config")
	}
}

func TestAuthUserInOneBotNotAnother(t *testing.T) {
	bot1Users := map[int64]*config.UserConfig{
		123456789: {WorkingDir: "/home/user"},
	}
	bot2Users := map[int64]*config.UserConfig{
		555555555: {WorkingDir: "/home/other"},
	}

	r1 := IsAuthorized(bot1Users, 123456789)
	if !r1.Authorized {
		t.Error("should be authorized in bot1")
	}

	r2 := IsAuthorized(bot2Users, 123456789)
	if r2.Authorized {
		t.Error("should not be authorized in bot2")
	}
}

func TestPrivateChatOnly(t *testing.T) {
	tests := map[string]bool{
		"private":    true,
		"group":      false,
		"supergroup": false,
		"channel":    false,
	}
	for chatType, want := range tests {
		t.Run(chatType, func(t *testing.T) {
			got := IsPrivateChat(chatType)
			if got != want {
				t.Errorf("IsPrivateChat(%q) = %v, want %v", chatType, got, want)
			}
		})
	}
}
