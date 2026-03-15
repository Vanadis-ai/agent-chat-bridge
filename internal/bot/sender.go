package bot

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

// TelegramSender abstracts Telegram API for testability.
type TelegramSender interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
	GetFileDirectURL(fileID string) (string, error)
	GetFile(config tgbotapi.FileConfig) (tgbotapi.File, error)
	Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error)
	GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
}
