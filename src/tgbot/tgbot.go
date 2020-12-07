package tgbot

import (
	"encoding/json"
	"net/http"

	api "github.com/go-telegram-bot-api/telegram-bot-api"
)

// TgBot allows you to interact with the Telegram Bot API.
type TgBot struct {
	*api.BotAPI
}

// Update is an update response, from GetUpdates.
type Update = api.Update

// NewTgBot creates a new TgBot instance.
//
// It requires a token, provided by @BotFather on Telegram.
func NewTgBot(token string) (*TgBot, error) {
	bot, err := api.NewBotAPI(token)
	return &TgBot{bot}, err
}

// ListenForWebhook registers a http handler for a webhook.
func (bot *TgBot) ListenForWebhook(pattern string, mux *http.ServeMux) api.UpdatesChannel {
	ch := make(chan api.Update, bot.Buffer)

	handler := func(w http.ResponseWriter, r *http.Request) {
		update, err := bot.HandleUpdate(r)
		if err != nil {
			errMsg, _ := json.Marshal(map[string]string{"error": err.Error()})
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(errMsg)
			return
		}

		ch <- *update
	}

	if mux != nil {
		mux.HandleFunc(pattern, handler)
	} else {
		http.HandleFunc(pattern, handler)
	}

	return ch
}

// MessageConfig contains information about a SendMessage request.
type MessageConfig = api.MessageConfig

// EditMessageTextConfig allows you to modify the text in a message.
type EditMessageTextConfig = api.EditMessageTextConfig

// DeleteMessageConfig contains information of a message in a chat to delete.
type DeleteMessageConfig = api.DeleteMessageConfig

// NewMessage creates a new Message.
//
// chatID is where to send it, text is the message text.
func NewMessage(chatID int64, text string) MessageConfig {
	msgConfig := api.NewMessage(chatID, text)
	msgConfig.ParseMode = "MarkdownV2"
	return msgConfig
}

// NewEditMessageText allows you to edit the text of a message.
func NewEditMessageText(chatID int64, messageID int, text string) EditMessageTextConfig {
	editMsgConfig := api.NewEditMessageText(chatID, messageID, text)
	editMsgConfig.ParseMode = "MarkdownV2"
	return editMsgConfig
}

// NewDeleteMessage creates a request to delete a message.
func NewDeleteMessage(chatID int64, messageID int) DeleteMessageConfig {
	return api.NewDeleteMessage(chatID, messageID)
}

// UpdatesChannel is the channel for getting updates.
type UpdatesChannel = api.UpdatesChannel
