package bot

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/vanadis-ai/agent-chat-bridge/internal/config"
	"github.com/vanadis-ai/agent-chat-bridge/internal/media"
)

// downloadResult holds the path where a file was saved.
type downloadResult struct {
	Path string
	Name string
}

// downloadFile downloads a Telegram file and saves it using the media package.
func (b *Bot) downloadFile(fileID, fileName, dir string, userCfg *config.UserConfig) (*downloadResult, error) {
	url, err := b.sender.GetFileDirectURL(fileID)
	if err != nil {
		return nil, fmt.Errorf("get file URL: %w", err)
	}

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download file: HTTP %d", resp.StatusCode)
	}

	quotaDirs := []string{userCfg.WorkingDir}
	path, err := media.Save(dir, fileName, resp.ContentLength, resp.Body, quotaDirs)
	if err != nil {
		return nil, fmt.Errorf("save file: %w", err)
	}

	return &downloadResult{Path: path, Name: fileName}, nil
}

// saveVoice downloads and saves a voice message.
func (b *Bot) saveVoice(voice *tgbotapi.Voice, userCfg *config.UserConfig) (*downloadResult, error) {
	name := fmt.Sprintf("voice_%d.ogg", voice.Duration)
	return b.downloadFile(voice.FileID, name, userCfg.VoiceDir, userCfg)
}

// saveAudio downloads and saves an audio file.
func (b *Bot) saveAudio(audio *tgbotapi.Audio, userCfg *config.UserConfig) (*downloadResult, error) {
	return b.downloadFile(audio.FileID, audio.FileName, userCfg.FilesDir, userCfg)
}

// saveDocument downloads and saves a document.
func (b *Bot) saveDocument(doc *tgbotapi.Document, userCfg *config.UserConfig) (*downloadResult, error) {
	return b.downloadFile(doc.FileID, doc.FileName, userCfg.FilesDir, userCfg)
}

// savePhoto downloads and saves the largest photo size.
func (b *Bot) savePhoto(photos []tgbotapi.PhotoSize, userCfg *config.UserConfig) (*downloadResult, error) {
	largest := photos[len(photos)-1]
	id := largest.FileID
	if len(id) > 8 {
		id = id[:8]
	}
	name := fmt.Sprintf("photo_%s.jpg", id)
	return b.downloadFile(largest.FileID, name, userCfg.FilesDir, userCfg)
}

// saveVideo downloads and saves a video.
func (b *Bot) saveVideo(video *tgbotapi.Video, userCfg *config.UserConfig) (*downloadResult, error) {
	name := video.FileName
	if name == "" {
		id := video.FileID
		if len(id) > 8 {
			id = id[:8]
		}
		name = fmt.Sprintf("video_%s.mp4", id)
	}
	return b.downloadFile(video.FileID, name, userCfg.FilesDir, userCfg)
}

// logDownload logs a successful file download.
func logDownload(bot, msgType, path string) {
	slog.Info("file saved", "bot", bot, "type", msgType, "path", path)
}

// logDownloadError logs a file download failure.
func logDownloadError(bot, msgType string, err error) {
	slog.Error("failed to save file", "bot", bot, "type", msgType, "error", err)
}

// discardBody is a no-op closer for when we don't need response body.
var _ io.ReadCloser
