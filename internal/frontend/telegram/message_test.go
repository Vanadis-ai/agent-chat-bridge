package telegram

import (
	"errors"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/vanadis-ai/agent-chat-bridge/internal/config"
	"github.com/vanadis-ai/agent-chat-bridge/internal/core"
)

func TestBuildChatMessageTextOnly(t *testing.T) {
	f := &Frontend{
		name:       "testbot",
		downloader: &mockDownloader{},
	}
	msg := &tgbotapi.Message{
		MessageID: 42,
		Chat:      &tgbotapi.Chat{ID: 100},
		From:      &tgbotapi.User{ID: 200},
		Text:      "hello world",
	}
	userCfg := &config.UserConfig{WorkingDir: "/home/user"}

	cm := f.buildChatMessage(msg, userCfg)

	if cm.ID != "42" {
		t.Errorf("ID = %q, want %q", cm.ID, "42")
	}
	if cm.ChatID != "100" {
		t.Errorf("ChatID = %q, want %q", cm.ChatID, "100")
	}
	if cm.UserID != "200" {
		t.Errorf("UserID = %q, want %q", cm.UserID, "200")
	}
	if cm.Text != "hello world" {
		t.Errorf("Text = %q, want %q", cm.Text, "hello world")
	}
	if cm.WorkingDir != "/home/user" {
		t.Errorf("WorkingDir = %q, want %q", cm.WorkingDir, "/home/user")
	}
	if len(cm.Attachments) != 0 {
		t.Errorf("Attachments count = %d, want 0", len(cm.Attachments))
	}
}

func TestBuildChatMessageVoice(t *testing.T) {
	dl := &mockDownloader{
		result: &downloadResult{Path: "/tmp/voice.ogg", Name: "voice_5.ogg"},
	}
	f := &Frontend{name: "testbot", downloader: dl}
	msg := &tgbotapi.Message{
		MessageID: 1,
		Chat:      &tgbotapi.Chat{ID: 100},
		From:      &tgbotapi.User{ID: 100},
		Voice:     &tgbotapi.Voice{FileID: "voice123", Duration: 5},
	}
	userCfg := &config.UserConfig{WorkingDir: "/tmp", VoiceDir: "/tmp/voice"}

	cm := f.buildChatMessage(msg, userCfg)

	if cm.Text == "" {
		t.Fatal("Text should not be empty for voice")
	}
	if len(cm.Attachments) != 1 {
		t.Fatalf("Attachments count = %d, want 1", len(cm.Attachments))
	}
	att := cm.Attachments[0]
	if att.Type != core.AttachmentVoice {
		t.Errorf("Type = %q, want %q", att.Type, core.AttachmentVoice)
	}
	if att.Duration != 5 {
		t.Errorf("Duration = %d, want 5", att.Duration)
	}
}

func TestBuildChatMessagePhotoWithCaption(t *testing.T) {
	dl := &mockDownloader{
		result: &downloadResult{Path: "/tmp/photo.jpg", Name: "photo_abc.jpg"},
	}
	f := &Frontend{name: "testbot", downloader: dl}
	msg := &tgbotapi.Message{
		MessageID: 1,
		Chat:      &tgbotapi.Chat{ID: 100},
		From:      &tgbotapi.User{ID: 100},
		Photo: []tgbotapi.PhotoSize{
			{FileID: "small", Width: 100, Height: 100},
			{FileID: "large", Width: 800, Height: 600},
		},
		Caption: "describe this",
	}
	userCfg := &config.UserConfig{WorkingDir: "/tmp", FilesDir: "/tmp/files"}

	cm := f.buildChatMessage(msg, userCfg)

	if len(cm.Attachments) != 1 {
		t.Fatalf("Attachments count = %d, want 1", len(cm.Attachments))
	}
	if cm.Attachments[0].Type != core.AttachmentImage {
		t.Errorf("Type = %q, want %q", cm.Attachments[0].Type, core.AttachmentImage)
	}
	if cm.Text == "" || cm.Text == "describe this" {
		t.Errorf("Text should contain file path and caption, got %q", cm.Text)
	}
}

func TestBuildChatMessageDownloadError(t *testing.T) {
	dl := &mockDownloader{err: errors.New("network error")}
	f := &Frontend{name: "testbot", downloader: dl}
	msg := &tgbotapi.Message{
		MessageID: 1,
		Chat:      &tgbotapi.Chat{ID: 100},
		From:      &tgbotapi.User{ID: 100},
		Photo: []tgbotapi.PhotoSize{
			{FileID: "photo123"},
		},
	}
	userCfg := &config.UserConfig{WorkingDir: "/tmp", FilesDir: "/tmp/files"}

	cm := f.buildChatMessage(msg, userCfg)

	if cm.Text == "" {
		t.Error("Text should contain error description")
	}
	if len(cm.Attachments) != 0 {
		t.Errorf("Attachments should be empty on download error, got %d", len(cm.Attachments))
	}
}

func TestBuildChatMessageDocument(t *testing.T) {
	dl := &mockDownloader{
		result: &downloadResult{Path: "/tmp/doc.pdf", Name: "doc.pdf"},
	}
	f := &Frontend{name: "testbot", downloader: dl}
	msg := &tgbotapi.Message{
		MessageID: 1,
		Chat:      &tgbotapi.Chat{ID: 100},
		From:      &tgbotapi.User{ID: 100},
		Document:  &tgbotapi.Document{FileID: "doc123", FileName: "doc.pdf", MimeType: "application/pdf"},
		Caption:   "review this",
	}
	userCfg := &config.UserConfig{WorkingDir: "/tmp", FilesDir: "/tmp/files"}

	cm := f.buildChatMessage(msg, userCfg)

	if len(cm.Attachments) != 1 {
		t.Fatalf("Attachments count = %d, want 1", len(cm.Attachments))
	}
	att := cm.Attachments[0]
	if att.Type != core.AttachmentDocument {
		t.Errorf("Type = %q, want %q", att.Type, core.AttachmentDocument)
	}
	if att.MimeType != "application/pdf" {
		t.Errorf("MimeType = %q, want %q", att.MimeType, "application/pdf")
	}
}

func TestBuildChatMessageUnsupported(t *testing.T) {
	f := &Frontend{name: "testbot", downloader: &mockDownloader{}}
	msg := &tgbotapi.Message{
		MessageID: 1,
		Chat:      &tgbotapi.Chat{ID: 100},
		From:      &tgbotapi.User{ID: 100},
		Sticker:   &tgbotapi.Sticker{FileID: "sticker123"},
	}
	userCfg := &config.UserConfig{WorkingDir: "/tmp"}

	cm := f.buildChatMessage(msg, userCfg)

	if cm.Text != "" {
		t.Errorf("Text = %q, want empty for unsupported type", cm.Text)
	}
}
