package bot

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/vanadis-ai/agent-chat-bridge/internal/claude"
	"github.com/vanadis-ai/agent-chat-bridge/internal/config"
)

func (b *Bot) handleUpdate(update tgbotapi.Update) {
	msg := update.Message
	if msg == nil {
		return
	}

	if !IsPrivateChat(msg.Chat.Type) {
		slog.Warn("rejected non-private chat",
			"bot", b.name, "chat_type", msg.Chat.Type, "chat_id", msg.Chat.ID)
		return
	}

	userID := msg.From.ID
	auth := IsAuthorized(b.config.Users, userID)
	if !auth.Authorized {
		slog.Warn("unauthorized user",
			"bot", b.name, "user_id", userID, "username", msg.From.UserName)
		b.sendText(msg.Chat.ID, "You are not authorized to use this bot.")
		return
	}

	slog.Info("incoming message",
		"bot", b.name,
		"user_id", userID,
		"msg_type", messageType(msg),
	)

	if msg.IsCommand() {
		b.handleCommand(msg)
		return
	}

	b.handleMessage(msg, auth.User)
}

func (b *Bot) handleMessage(msg *tgbotapi.Message, userCfg *config.UserConfig) {
	chatID := msg.Chat.ID
	userID := msg.From.ID

	if b.hasActive(userID) {
		b.sendText(chatID, "Previous request is still running. Use /stop to cancel it.")
		return
	}

	prompt := b.buildPrompt(msg, userCfg)
	if prompt == "" {
		b.sendText(chatID, "Received unsupported message type")
		return
	}

	go b.runClaude(chatID, userID, prompt, userCfg)
}

func (b *Bot) buildPrompt(msg *tgbotapi.Message, userCfg *config.UserConfig) string {
	switch {
	case msg.Voice != nil:
		return b.buildVoicePrompt(msg, userCfg)
	case msg.Audio != nil:
		return b.buildAudioPrompt(msg, userCfg)
	case msg.Document != nil:
		return b.buildDocumentPrompt(msg, userCfg)
	case msg.Photo != nil:
		return b.buildPhotoPrompt(msg, userCfg)
	case msg.Video != nil:
		return b.buildVideoPrompt(msg, userCfg)
	case msg.Text != "":
		return msg.Text
	default:
		return ""
	}
}

func (b *Bot) buildVoicePrompt(msg *tgbotapi.Message, userCfg *config.UserConfig) string {
	res, err := b.saveVoice(msg.Voice, userCfg)
	if err != nil {
		logDownloadError(b.name, "voice", err)
		return fmt.Sprintf("[Voice message %ds, failed to download]", msg.Voice.Duration)
	}
	logDownload(b.name, "voice", res.Path)
	return fmt.Sprintf("[Voice message saved to %s, duration %ds]", res.Path, msg.Voice.Duration)
}

func (b *Bot) buildAudioPrompt(msg *tgbotapi.Message, userCfg *config.UserConfig) string {
	res, err := b.saveAudio(msg.Audio, userCfg)
	if err != nil {
		logDownloadError(b.name, "audio", err)
		return fmt.Sprintf("[Audio file: %s, failed to download]", msg.Audio.FileName)
	}
	logDownload(b.name, "audio", res.Path)
	prompt := fmt.Sprintf("[Audio file saved to %s]", res.Path)
	if msg.Caption != "" {
		prompt += "\n" + msg.Caption
	}
	return prompt
}

func (b *Bot) buildDocumentPrompt(msg *tgbotapi.Message, userCfg *config.UserConfig) string {
	res, err := b.saveDocument(msg.Document, userCfg)
	if err != nil {
		logDownloadError(b.name, "document", err)
		return fmt.Sprintf("[Document: %s, failed to download]", msg.Document.FileName)
	}
	logDownload(b.name, "document", res.Path)
	prompt := fmt.Sprintf("[Document saved to %s]", res.Path)
	if msg.Caption != "" {
		prompt += "\n" + msg.Caption
	}
	return prompt
}

func (b *Bot) buildPhotoPrompt(msg *tgbotapi.Message, userCfg *config.UserConfig) string {
	res, err := b.savePhoto(msg.Photo, userCfg)
	if err != nil {
		logDownloadError(b.name, "photo", err)
		return "[Photo, failed to download]"
	}
	logDownload(b.name, "photo", res.Path)
	prompt := fmt.Sprintf("[Photo saved to %s]", res.Path)
	if msg.Caption != "" {
		prompt += "\n" + msg.Caption
	}
	return prompt
}

func (b *Bot) buildVideoPrompt(msg *tgbotapi.Message, userCfg *config.UserConfig) string {
	res, err := b.saveVideo(msg.Video, userCfg)
	if err != nil {
		logDownloadError(b.name, "video", err)
		return "[Video, failed to download]"
	}
	logDownload(b.name, "video", res.Path)
	prompt := fmt.Sprintf("[Video saved to %s]", res.Path)
	if msg.Caption != "" {
		prompt += "\n" + msg.Caption
	}
	return prompt
}

func (b *Bot) runClaude(chatID, userID int64, prompt string, userCfg *config.UserConfig) {
	timeout := time.Duration(b.claude.TimeoutMinutes) * time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	defer b.clearActive(userID)

	if !b.setActive(userID, cancel) {
		b.sendText(chatID, "Previous request is still running. Use /stop to cancel it.")
		return
	}

	streamer := NewStreamer(b.sender, chatID)
	if err := streamer.SendInitial(); err != nil {
		slog.Error("failed to send initial message", "bot", b.name, "error", err)
		return
	}

	deltaCh := make(chan claude.StreamDelta, 100)
	go func() {
		for delta := range deltaCh {
			streamer.Append(delta.Text)
		}
	}()

	cfg := claude.RunConfig{
		Prompt:         prompt,
		WorkingDir:     userCfg.WorkingDir,
		Model:          b.config.Model,
		PermissionMode: b.config.PermissionMode,
		SystemPrompt:   b.config.AppendSystemPrompt,
		SessionID:      b.sessions.Get(userID),
		CLIPath:        b.claude.Binary,
		TimeoutMinutes: b.claude.TimeoutMinutes,
	}

	result, err := claude.Run(ctx, cfg, deltaCh)
	streamer.Finalize()

	if err != nil {
		slog.Error("claude run failed", "bot", b.name, "error", err)
		b.sendText(chatID, fmt.Sprintf("Error: %s", err))
		return
	}

	if result.IsError {
		slog.Error("claude returned error", "bot", b.name, "error", result.Error)
		b.sendText(chatID, fmt.Sprintf("Claude error: %s", result.Error))
		return
	}

	if result.SessionID != "" {
		b.sessions.Set(userID, result.SessionID)
	}

	slog.Info("claude request complete",
		"bot", b.name,
		"user_id", userID,
		"session", result.SessionID,
		"cost_usd", result.CostUSD,
		"turns", result.NumTurns,
	)
}

func messageType(msg *tgbotapi.Message) string {
	switch {
	case msg.Voice != nil:
		return "voice"
	case msg.Audio != nil:
		return "audio"
	case msg.Document != nil:
		return "document"
	case msg.Photo != nil:
		return "photo"
	case msg.Video != nil:
		return "video"
	case msg.VideoNote != nil:
		return "video_note"
	case msg.Sticker != nil:
		return "sticker"
	case msg.Location != nil:
		return "location"
	case msg.Contact != nil:
		return "contact"
	case msg.Text != "":
		return "text"
	default:
		return "unknown"
	}
}
