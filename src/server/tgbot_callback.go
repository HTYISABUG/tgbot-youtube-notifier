package server

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/HTYISABUG/tgbot-youtube-notifier/src/tgbot"
	"github.com/golang/glog"
)

type CallbackDataType int

const (
	Record CallbackDataType = iota
	List
	Operation
	Remove
)

type OperationType int

const (
	RemoveOp OperationType = iota
	BackOp
)

func (s *Server) callbackHandler(update tgbot.Update) {
	// Basic callback info
	callbackID := update.CallbackQuery.ID

	// Decode callback data
	data := make(map[string]interface{})
	json.Unmarshal([]byte(update.CallbackQuery.Data), &data)

	var err error

	switch CallbackDataType(data["type"].(float64)) {
	case Record:
		err = s.callbackRecordHandler(update)
	case List:
		err = s.callbackListHandler(update)
	case Operation:
		err = s.callbackOpHandler(update)
	case Remove:
		err = s.callbackRemoveHandler(update)
	default:
		err = fmt.Errorf("invalid callback type: %v", data["type"])
	}

	if err != nil {
		s.internalServerErrorCallback(callbackID)
		glog.Error(err)
	}
}

func (s *Server) newRecordButtonMarkup(videoID string) (*tgbot.InlineKeyboardMarkup, error) {
	data := make(map[string]interface{})
	data["type"] = Record
	data["videoID"] = videoID
	b, _ := json.Marshal(data)

	button := tgbot.NewInlineKeyboardButtonData("Record", string(b))
	row := tgbot.NewInlineKeyboardRow(button)
	markup := tgbot.NewInlineKeyboardMarkup(row)

	return &markup, nil
}

func (s *Server) callbackRecordHandler(update tgbot.Update) error {
	// Basic callback info
	callbackID := update.CallbackQuery.ID
	chatID := update.CallbackQuery.Message.Chat.ID
	msgID := update.CallbackQuery.Message.MessageID

	// Decode callback data
	var data struct {
		VideoID string `json:"videoID"`
	}

	json.Unmarshal([]byte(update.CallbackQuery.Data), &data)

	if _, err := s.db.Exec(
		"INSERT IGNORE INTO records (chatID, videoID) VALUES (?, ?);",
		chatID, data.VideoID,
	); err != nil {
		return err
	}

	callback := tgbot.NewCallback(callbackID, fmt.Sprintf("Add %s recorder", data.VideoID))
	s.tg.AnswerCallbackQuery(callback)

	cfg := tgbot.NewEditMessageReplyMarkup(chatID, msgID, tgbot.InlineKeyboardMarkup{InlineKeyboard: [][]tgbot.InlineKeyboardButton{{}}})
	s.tgSend(cfg)

	return nil
}

func (s *Server) newChannelListMarkUp(chatID int64, page int) (*tgbot.InlineKeyboardMarkup, error) {
	const MAX_LIST_LENGTH = 5

	channels, err := s.db.getChannelsByChatID(chatID)
	if err != nil {
		return nil, err
	} else {
		var rows [][]tgbot.InlineKeyboardButton

		// Limited list length
		offset := page * MAX_LIST_LENGTH
		length := len(channels) - offset
		if length > MAX_LIST_LENGTH {
			length = MAX_LIST_LENGTH
		}

		for i := offset; i < offset+length; i++ {
			data := make(map[string]interface{})
			data["type"] = List
			data["cid"] = channels[i].id
			data["page"] = page
			b, _ := json.Marshal(data)

			button := tgbot.NewInlineKeyboardButtonData(channels[i].title, string(b))
			row := tgbot.NewInlineKeyboardRow(button)
			rows = append(rows, row)
		}

		var buttons []tgbot.InlineKeyboardButton

		if page != 0 {
			data := make(map[string]interface{})
			data["type"] = List
			data["page"] = page - 1
			b, _ := json.Marshal(data)
			button := tgbot.NewInlineKeyboardButtonData("←", string(b))
			buttons = append(buttons, button)
		}

		if len(channels)-offset > MAX_LIST_LENGTH {
			data := make(map[string]interface{})
			data["type"] = List
			data["page"] = page + 1
			b, _ := json.Marshal(data)
			button := tgbot.NewInlineKeyboardButtonData("→", string(b))
			buttons = append(buttons, button)
		}

		if len(buttons) != 0 {
			row := tgbot.NewInlineKeyboardRow(buttons...)
			rows = append(rows, row)
		}

		markup := tgbot.NewInlineKeyboardMarkup(rows...)

		return &markup, nil
	}
}

func (s *Server) callbackListHandler(update tgbot.Update) error {
	// Basic callback info
	chatID := update.CallbackQuery.Message.Chat.ID
	msgID := update.CallbackQuery.Message.MessageID

	// Decode callback data
	var data struct {
		ChannelID string `json:"cid"`
		Page      int    `json:"page"`
	}

	json.Unmarshal([]byte(update.CallbackQuery.Data), &data)

	if data.ChannelID == "" {
		// Turn page
		markup, err := s.newChannelListMarkUp(chatID, data.Page)
		if err != nil {
			return err
		}

		cfg := tgbot.NewEditMessageReplyMarkup(chatID, msgID, *markup)
		s.tgSend(cfg)
	} else {
		// Subscribed channel operation
		markup, err := s.newChannelOpMarkUp(data.ChannelID, data.Page)
		if err != nil {
			return err
		}

		// Get channel title
		var channelTitle string
		err = s.db.QueryRow("SELECT title FROM channels WHERE id = ?;", data.ChannelID).Scan(&channelTitle)
		if err != nil {
			return err
		}

		link := tgbot.InlineLink(tgbot.EscapeText(channelTitle), "https://www.youtube.com/channel/"+data.ChannelID)
		cfg := tgbot.NewEditMessageTextAndMarkup(chatID, msgID, fmt.Sprintf("Here it is: %s\nWhat do you want to do with the channel?", link), *markup)
		s.tgSend(cfg)
	}

	return nil
}

func (s *Server) newChannelOpMarkUp(channelID string, page int) (*tgbot.InlineKeyboardMarkup, error) {
	var data map[string]interface{}
	var b []byte
	var buttons [][]tgbot.InlineKeyboardButton

	// Construct `remove` button
	data = make(map[string]interface{})
	data["type"] = Operation
	data["cid"] = channelID
	data["op"] = RemoveOp
	data["page"] = page

	b, _ = json.Marshal(data)
	remove := tgbot.NewInlineKeyboardButtonData("Remove", string(b))
	buttons = append(buttons, tgbot.NewInlineKeyboardRow(remove))

	// Construct `back` button
	data = make(map[string]interface{})
	data["type"] = Operation
	data["op"] = BackOp
	data["page"] = page

	b, _ = json.Marshal(data)
	back := tgbot.NewInlineKeyboardButtonData("« Back to Channel List", string(b))
	buttons = append(buttons, tgbot.NewInlineKeyboardRow(back))

	markup := tgbot.NewInlineKeyboardMarkup(buttons...)

	return &markup, nil
}

func (s *Server) callbackOpHandler(update tgbot.Update) error {
	// Basic callback info
	chatID := update.CallbackQuery.Message.Chat.ID
	msgID := update.CallbackQuery.Message.MessageID

	// Decode callback data
	var data struct {
		ChannelID string        `json:"cid"`
		Op        OperationType `json:"op"`
		Page      int           `json:"page"`
	}

	json.Unmarshal([]byte(update.CallbackQuery.Data), &data)

	switch data.Op {
	case RemoveOp:
		markup, err := s.newChannelRemoveMarkUp(data.ChannelID, data.Page)
		if err != nil {
			return err
		}

		// Get channel title
		var channelTitle string
		err = s.db.QueryRow("SELECT title FROM channels WHERE id = ?;", data.ChannelID).Scan(&channelTitle)
		if err != nil {
			return err
		}

		link := tgbot.InlineLink(tgbot.EscapeText(channelTitle), "https://www.youtube.com/channel/"+data.ChannelID)
		cfg := tgbot.NewEditMessageTextAndMarkup(chatID, msgID, fmt.Sprintf("Do you really want to remove %s?", link), *markup)
		s.tgSend(cfg)
	case BackOp:
		markup, err := s.newChannelListMarkUp(chatID, data.Page)
		if err != nil {
			return err
		}

		cfg := tgbot.NewEditMessageTextAndMarkup(chatID, msgID, "You already subscribed following channels:", *markup)
		s.tgSend(cfg)
	}

	return nil
}

func (s *Server) newChannelRemoveMarkUp(channelID string, page int) (*tgbot.InlineKeyboardMarkup, error) {
	var data map[string]interface{}
	var b []byte

	// Construct `remove` button
	data = make(map[string]interface{})
	data["type"] = Remove
	data["cid"] = channelID

	b, _ = json.Marshal(data)
	remove := tgbot.NewInlineKeyboardButtonData("Remove", string(b))

	// Construct `cancel` button
	data = make(map[string]interface{})
	data["type"] = List
	data["cid"] = channelID
	data["page"] = page

	b, _ = json.Marshal(data)
	cancel := tgbot.NewInlineKeyboardButtonData("Cancel", string(b))

	row := tgbot.NewInlineKeyboardRow(remove, cancel)
	markup := tgbot.NewInlineKeyboardMarkup(row)

	return &markup, nil
}

func (s *Server) callbackRemoveHandler(update tgbot.Update) error {
	// Basic callback info
	chatID := update.CallbackQuery.Message.Chat.ID
	msgID := update.CallbackQuery.Message.MessageID

	// Decode callback data
	var data struct {
		ChannelID string `json:"cid"`
	}

	json.Unmarshal([]byte(update.CallbackQuery.Data), &data)

	// Get channel title
	var channelTitle string
	err := s.db.QueryRow("SELECT title FROM channels WHERE id = ?;", data.ChannelID).Scan(&channelTitle)
	if err != nil {
		return err
	}

	// Remove subscription
	_, err = s.db.Exec("DELETE FROM subscribers WHERE chatID = ? AND channelID = ?;", chatID, data.ChannelID)
	if err != nil {
		return err
	}

	link := tgbot.InlineLink(tgbot.EscapeText(channelTitle), "https://www.youtube.com/channel/"+data.ChannelID)
	cfg := tgbot.NewEditMessageText(chatID, msgID, fmt.Sprintf("You already unsubscribe\n%s", link))
	s.tgSend(cfg)

	// Check not subscribed channels & unsubscribe them from hub
	go func() {
		var channelIDs []string

		err := s.db.queryResults(
			&channelIDs,
			func(rows *sql.Rows, dest interface{}) error {
				r := dest.(*string)
				return rows.Scan(r)
			},
			"SELECT channels.id FROM "+
				"channels LEFT JOIN subscribers ON channels.id = subscribers.channelID "+
				"WHERE subscribers.chatID IS NULL;",
		)

		if err != nil {
			glog.Error(err)
			return
		}

		for _, id := range channelIDs {
			if _, err := s.db.Exec("DELETE FROM channels WHERE id = ?;", id); err != nil {
				glog.Error(err)
				continue
			}

			s.hub.Unsubscribe(id)
		}
	}()
	return nil
}
