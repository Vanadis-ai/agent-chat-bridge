package telegram

import (
	"fmt"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/vanadis-ai/agent-chat-bridge/internal/config"
	"github.com/vanadis-ai/agent-chat-bridge/internal/core"
)

// buildChatMessage converts a Telegram message to a platform-agnostic ChatMessage.
// Populates both Text (with embedded file paths) and Attachments (structural).
func (f *Frontend) buildChatMessage(
	msg *tgbotapi.Message,
	userCfg *config.UserConfig,
) core.ChatMessage {
	cm := core.ChatMessage{
		ID:         strconv.Itoa(msg.MessageID),
		ChatID:     formatID(msg.Chat.ID),
		UserID:     formatID(msg.From.ID),
		WorkingDir: userCfg.WorkingDir,
		IsCommand:  msg.IsCommand(),
		Command:    msg.Command(),
	}

	switch {
	case msg.Voice != nil:
		cm.Text, cm.Attachments = f.buildVoiceMessage(msg.Voice, userCfg)
	case msg.Audio != nil:
		cm.Text, cm.Attachments = f.buildAudioMessage(msg.Audio, msg.Caption, userCfg)
	case msg.Document != nil:
		cm.Text, cm.Attachments = f.buildDocumentMessage(msg.Document, msg.Caption, userCfg)
	case msg.Photo != nil:
		cm.Text, cm.Attachments = f.buildPhotoMessage(msg.Photo, msg.Caption, userCfg)
	case msg.Video != nil:
		cm.Text, cm.Attachments = f.buildVideoMessage(msg.Video, msg.Caption, userCfg)
	case msg.Text != "":
		cm.Text = msg.Text
	}
	return cm
}

func (f *Frontend) buildVoiceMessage(
	voice *tgbotapi.Voice, userCfg *config.UserConfig,
) (string, []core.Attachment) {
	res, err := f.downloader.saveVoice(voice, userCfg)
	if err != nil {
		logDownloadError(f.name, "voice", err)
		return fmt.Sprintf("[Voice message %ds, failed to download]", voice.Duration), nil
	}
	logDownload(f.name, "voice", res.Path)
	att := core.Attachment{
		Type: core.AttachmentVoice, Path: res.Path, Name: res.Name,
		Duration: voice.Duration,
	}
	return fmt.Sprintf("[Voice message saved to %s, duration %ds]", res.Path, voice.Duration), []core.Attachment{att}
}

func (f *Frontend) buildAudioMessage(
	audio *tgbotapi.Audio, caption string, userCfg *config.UserConfig,
) (string, []core.Attachment) {
	res, err := f.downloader.saveAudio(audio, userCfg)
	if err != nil {
		logDownloadError(f.name, "audio", err)
		return fmt.Sprintf("[Audio file: %s, failed to download]", audio.FileName), nil
	}
	logDownload(f.name, "audio", res.Path)
	att := core.Attachment{
		Type: core.AttachmentAudio, Path: res.Path, Name: res.Name,
		MimeType: audio.MimeType, Duration: audio.Duration,
	}
	text := fmt.Sprintf("[Audio file saved to %s]", res.Path)
	if caption != "" {
		text += "\n" + caption
	}
	return text, []core.Attachment{att}
}

func (f *Frontend) buildDocumentMessage(
	doc *tgbotapi.Document, caption string, userCfg *config.UserConfig,
) (string, []core.Attachment) {
	res, err := f.downloader.saveDocument(doc, userCfg)
	if err != nil {
		logDownloadError(f.name, "document", err)
		return fmt.Sprintf("[Document: %s, failed to download]", doc.FileName), nil
	}
	logDownload(f.name, "document", res.Path)
	att := core.Attachment{
		Type: core.AttachmentDocument, Path: res.Path, Name: res.Name,
		MimeType: doc.MimeType, Size: int64(doc.FileSize),
	}
	text := fmt.Sprintf("[Document saved to %s]", res.Path)
	if caption != "" {
		text += "\n" + caption
	}
	return text, []core.Attachment{att}
}

func (f *Frontend) buildPhotoMessage(
	photos []tgbotapi.PhotoSize, caption string, userCfg *config.UserConfig,
) (string, []core.Attachment) {
	res, err := f.downloader.savePhoto(photos, userCfg)
	if err != nil {
		logDownloadError(f.name, "photo", err)
		return "[Photo, failed to download]", nil
	}
	logDownload(f.name, "photo", res.Path)
	largest := photos[len(photos)-1]
	att := core.Attachment{
		Type: core.AttachmentImage, Path: res.Path, Name: res.Name,
		Size: int64(largest.FileSize),
	}
	text := fmt.Sprintf("[Photo saved to %s]", res.Path)
	if caption != "" {
		text += "\n" + caption
	}
	return text, []core.Attachment{att}
}

func (f *Frontend) buildVideoMessage(
	video *tgbotapi.Video, caption string, userCfg *config.UserConfig,
) (string, []core.Attachment) {
	res, err := f.downloader.saveVideo(video, userCfg)
	if err != nil {
		logDownloadError(f.name, "video", err)
		return "[Video, failed to download]", nil
	}
	logDownload(f.name, "video", res.Path)
	att := core.Attachment{
		Type: core.AttachmentVideo, Path: res.Path, Name: res.Name,
		Duration: video.Duration,
	}
	text := fmt.Sprintf("[Video saved to %s]", res.Path)
	if caption != "" {
		text += "\n" + caption
	}
	return text, []core.Attachment{att}
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
	case msg.Text != "":
		return "text"
	default:
		return "unknown"
	}
}
