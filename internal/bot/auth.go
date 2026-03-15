package bot

import "github.com/vanadis-ai/agent-chat-bridge/internal/config"

// AuthResult holds the result of an authorization check.
type AuthResult struct {
	User       *config.UserConfig
	Authorized bool
}

// IsAuthorized checks if a user ID exists in the bot's user map.
func IsAuthorized(users map[int64]*config.UserConfig, userID int64) AuthResult {
	u, ok := users[userID]
	if !ok {
		return AuthResult{Authorized: false}
	}
	return AuthResult{User: u, Authorized: true}
}

// IsPrivateChat returns true only for private (non-group) chats.
func IsPrivateChat(chatType string) bool {
	return chatType == "private"
}
