package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/vanadis-ai/agent-chat-bridge/internal/config"
	"github.com/vanadis-ai/agent-chat-bridge/internal/core"
)

func (f *Frontend) handleUpdate(ctx context.Context, update tgbotapi.Update) {
	msg := update.Message
	if msg == nil {
		return
	}
	if !IsPrivateChat(msg.Chat.Type) {
		slog.Warn("rejected non-private chat",
			"bot", f.name, "chat_type", msg.Chat.Type, "chat_id", msg.Chat.ID)
		return
	}

	userID := msg.From.ID
	auth := IsAuthorized(f.config.Users, userID)
	if !auth.Authorized {
		slog.Warn("unauthorized user",
			"bot", f.name, "user_id", userID, "username", msg.From.UserName)
		f.sendText(msg.Chat.ID, "You are not authorized to use this bot.")
		return
	}

	slog.Info("incoming message",
		"bot", f.name, "user_id", userID, "msg_type", messageType(msg))

	if msg.IsCommand() {
		f.handleCommand(msg)
		return
	}

	f.handleMessage(ctx, msg, auth.User)
}

func (f *Frontend) handleMessage(ctx context.Context, msg *tgbotapi.Message, userCfg *config.UserConfig) {
	chatMsg := f.buildChatMessage(msg, userCfg)
	if chatMsg.Text == "" {
		f.sendText(msg.Chat.ID, "Received unsupported message type")
		return
	}
	go f.runRequest(ctx, msg.Chat.ID, chatMsg)
}

func (f *Frontend) runRequest(ctx context.Context, chatID int64, chatMsg core.ChatMessage) {
	if f.bridge.HasActive(f.name, chatMsg.ChatID, chatMsg.UserID) {
		f.sendText(chatID, "Previous request is still running. Use /stop to cancel it.")
		return
	}

	streamer := NewStreamer(f.sender, chatID)
	if err := streamer.SendInitial(); err != nil {
		slog.Error("failed to send initial message", "bot", f.name, "error", err)
		return
	}

	deltaCh := make(chan core.StreamDelta, 100)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for delta := range deltaCh {
			streamer.Append(delta.Text)
		}
	}()

	resp, err := f.handler(ctx, chatMsg, deltaCh)
	wg.Wait()
	streamer.Finalize()

	f.handleResult(chatID, chatMsg.UserID, streamer, resp, err)
}

func (f *Frontend) handleResult(chatID int64, userID string, streamer *Streamer, resp *core.Response, err error) {
	if errors.Is(err, core.ErrRequestActive) {
		streamer.EditPlaceholder("Previous request is still running. Use /stop to cancel it.")
		return
	}
	if err != nil {
		slog.Error("request failed", "bot", f.name, "error", err)
		streamer.EditPlaceholder(fmt.Sprintf("Error: %s", err))
		return
	}
	if resp == nil {
		return
	}
	if resp.IsError {
		slog.Error("backend returned error", "bot", f.name, "error", resp.Error)
		streamer.EditPlaceholder(fmt.Sprintf("Error: %s", resp.Error))
		return
	}
	// If backend/plugin returned Response.Text but no deltas were streamed,
	// show the text by editing the placeholder.
	if resp.Text != "" && !streamer.HasContent() {
		streamer.EditPlaceholder(resp.Text)
	}
	slog.Info("request complete",
		"bot", f.name,
		"user_id", userID,
		"session", resp.SessionID,
		"cost_usd", resp.CostUSD,
		"turns", resp.NumTurns,
	)
}

func formatID(id int64) string {
	return strconv.FormatInt(id, 10)
}
