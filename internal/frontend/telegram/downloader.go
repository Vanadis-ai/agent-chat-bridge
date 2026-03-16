package telegram

import (
	"fmt"
	"log/slog"
	"net/http"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/vanadis-ai/agent-chat-bridge/internal/config"
	"github.com/vanadis-ai/agent-chat-bridge/internal/media"
)

// fileDownloader abstracts file download operations for testability.
type fileDownloader interface {
	saveVoice(voice *tgbotapi.Voice, userCfg *config.UserConfig) (*downloadResult, error)
	saveAudio(audio *tgbotapi.Audio, userCfg *config.UserConfig) (*downloadResult, error)
	saveDocument(doc *tgbotapi.Document, userCfg *config.UserConfig) (*downloadResult, error)
	savePhoto(photos []tgbotapi.PhotoSize, userCfg *config.UserConfig) (*downloadResult, error)
	saveVideo(video *tgbotapi.Video, userCfg *config.UserConfig) (*downloadResult, error)
}

// downloadResult holds the path where a file was saved.
type downloadResult struct {
	Path string
	Name string
}

// Downloader downloads Telegram files and saves them using the media package.
type Downloader struct {
	sender  TelegramSender
	botName string
}

func newDownloader(sender TelegramSender, botName string) *Downloader {
	return &Downloader{sender: sender, botName: botName}
}

func (d *Downloader) downloadFile(fileID, fileName, dir string, userCfg *config.UserConfig) (*downloadResult, error) {
	url, err := d.sender.GetFileDirectURL(fileID)
	if err != nil {
		return nil, fmt.Errorf("get file URL: %w", err)
	}
	resp, err := http.Get(url) //nolint:gosec // URL from Telegram API
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

func (d *Downloader) saveVoice(voice *tgbotapi.Voice, userCfg *config.UserConfig) (*downloadResult, error) {
	name := fmt.Sprintf("voice_%d.ogg", voice.Duration)
	return d.downloadFile(voice.FileID, name, userCfg.VoiceDir, userCfg)
}

func (d *Downloader) saveAudio(audio *tgbotapi.Audio, userCfg *config.UserConfig) (*downloadResult, error) {
	return d.downloadFile(audio.FileID, audio.FileName, userCfg.FilesDir, userCfg)
}

func (d *Downloader) saveDocument(doc *tgbotapi.Document, userCfg *config.UserConfig) (*downloadResult, error) {
	return d.downloadFile(doc.FileID, doc.FileName, userCfg.FilesDir, userCfg)
}

func (d *Downloader) savePhoto(photos []tgbotapi.PhotoSize, userCfg *config.UserConfig) (*downloadResult, error) {
	largest := photos[len(photos)-1]
	name := fmt.Sprintf("photo_%s.jpg", shortID(largest.FileID))
	return d.downloadFile(largest.FileID, name, userCfg.FilesDir, userCfg)
}

func (d *Downloader) saveVideo(video *tgbotapi.Video, userCfg *config.UserConfig) (*downloadResult, error) {
	name := video.FileName
	if name == "" {
		name = fmt.Sprintf("video_%s.mp4", shortID(video.FileID))
	}
	return d.downloadFile(video.FileID, name, userCfg.FilesDir, userCfg)
}

func shortID(fileID string) string {
	if len(fileID) > 8 {
		return fileID[:8]
	}
	return fileID
}

func logDownload(bot, msgType, path string) {
	slog.Info("file saved", "bot", bot, "type", msgType, "path", path)
}

func logDownloadError(bot, msgType string, err error) {
	slog.Error("failed to save file", "bot", bot, "type", msgType, "error", err)
}
