package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/HTYISABUG/tgbot-youtube-notifier/src/hub"
	"github.com/HTYISABUG/tgbot-youtube-notifier/src/tgbot"
	"github.com/HTYISABUG/tgbot-youtube-notifier/src/ytapi"
	"github.com/golang/glog"
)

func (s *Server) tgSend(c tgbot.Chattable) {
	_, err := s.tg.Send(c)
	if err != nil {
		switch err.(type) {
		case tgbot.Error:
			switch cfg := c.(type) {
			case tgbot.EditMessageTextConfig, tgbot.EditMessageReplyMarkupConfig:
				const notModified = "message is not modified"

				if !strings.Contains(err.Error(), notModified) {
					glog.Error(err)
					fmt.Printf("%+v\n", cfg)
					debug.PrintStack()
				}
			default:
				glog.Error(err)
				fmt.Printf("%+v\n", cfg)
				debug.PrintStack()
			}
		default:
			glog.Warning(err)
		}
	}
}

// chAddHandler handles channel subscribe request.
func (s *Server) chAddHandler(update tgbot.Update) {
	chatID := update.Message.Chat.ID
	elements := strings.Fields(update.Message.Text)

	// Command with out parameters.
	if len(elements) == 1 {
		msgConfig := tgbot.NewMessage(
			chatID,
			"Please use "+tgbot.InlineCode("/add <channel url> ...")+" to subscribe\\.",
		)

		s.tgSend(msgConfig)
		return
	}

	// Loop over every parameters.
	for _, e := range elements[1:] {
		var title string = e
		var msgTemplate string
		var msgConfig tgbot.MessageConfig

		// Validation url parameter.
		if b, err := isValidYtChannel(e); err == nil && b {
			// If e is a valid yt channel...
			_, url, _ := followRedirectURL(e)
			channelID := strings.Split(url.Path, "/")[2]

			// Get channel snippet from YouTube.
			c, err := s.yt.GetChannel(channelID, []string{"snippet"})
			if err != nil {
				switch err.(type) {
				case ytapi.InvalidChannelIDError:
					msgTemplate = "%s %s failed.\n" + fmt.Sprintf("Invalid channel ID: %s.", channelID)
				default:
					glog.Warning(err)
					msgTemplate = "%s %s failed.\nInternal server error."
				}
			} else {
				title = c.Snippet.Title
				// Insert into database.
				if err := s.db.subscribe(chatID, Channel{id: c.Id, title: c.Snippet.Title}); err != nil {
					glog.Warning(err)
					msgTemplate = "%s %s failed.\nInternal server error."
				}
			}

			// Run subscription
			if msgTemplate == "" {
				s.hub.Subscribe(c.Id)
				msgTemplate = "%s %s successful."
			}

			title = tgbot.EscapeText(title)
			msgTemplate = tgbot.EscapeText(msgTemplate)
			msgConfig = tgbot.NewMessage(chatID, fmt.Sprintf(
				msgTemplate,
				tgbot.ItalicText(tgbot.BordText("Subscribe")),
				tgbot.InlineLink(title, e),
			))
		} else if err != nil {
			// If valid check failed...
			glog.Warning(err)
			msgConfig = tgbot.NewMessage(chatID, fmt.Sprintf(
				"Subscribe %s failed, internal server error",
				tgbot.EscapeText(e),
			))
		} else if !b {
			// If e isn't a valid yt channel...
			msgConfig = tgbot.NewMessage(chatID, fmt.Sprintf(
				"%s is not a valid YouTube channel",
				tgbot.EscapeText(e),
			))
		}

		msgConfig.DisableNotification = true
		msgConfig.DisableWebPagePreview = true

		s.tgSend(msgConfig)
	}
}

// chListHandler handles list subscribed channels request.
func (s *Server) chListHandler(update tgbot.Update) {
	chatID := update.Message.Chat.ID

	var msgConfig tgbot.MessageConfig
	defer func() {
		msgConfig.DisableNotification = true
		msgConfig.DisableWebPagePreview = true
		s.tgSend(msgConfig)
	}()

	markup, err := s.newChannelListMarkUp(chatID, 0)
	if err != nil {
		glog.Error(err)
		msgConfig = tgbot.NewMessage(chatID, "Can not list subscribed channels.\nInternal server error.")
	} else {
		msgConfig = tgbot.NewMessage(chatID, "You already subscribed following channels:")
		msgConfig.ReplyMarkup = markup
	}
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
			data["type"] = "list"
			data["page"] = page
			data["channelID"] = channels[i].id
			b, _ := json.Marshal(data)

			button := tgbot.NewInlineKeyboardButtonData(channels[i].title, string(b))
			row := tgbot.NewInlineKeyboardRow(button)
			rows = append(rows, row)
		}

		var buttons []tgbot.InlineKeyboardButton

		if page != 0 {
			data := make(map[string]interface{})
			data["type"] = "list"
			data["page"] = page - 1
			b, _ := json.Marshal(data)
			button := tgbot.NewInlineKeyboardButtonData("←", string(b))
			buttons = append(buttons, button)
		}

		if len(channels)-offset > MAX_LIST_LENGTH {
			data := make(map[string]interface{})
			data["type"] = "list"
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

// chRemoveHandler handles channel unsubscribe request.
func (s *Server) chRemoveHandler(update tgbot.Update) {
	chatID := update.Message.Chat.ID
	elements := strings.Fields(update.Message.Text)

	var msgConfig tgbot.MessageConfig
	defer func() {
		msgConfig.DisableNotification = true
		msgConfig.DisableWebPagePreview = true
		s.tgSend(msgConfig)
	}()

	if len(elements) == 1 {
		msgConfig = tgbot.NewMessage(
			chatID,
			"Please use /list to find the channel numbers which you want to unsubscribe\\. "+
				"Then use "+tgbot.InlineCode("/remove <number> ...")+" to unsubscribe\\.",
		)
		return
	}

	channels, err := s.db.getChannelsByChatID(chatID)
	if err != nil {
		glog.Error(err)
		msgConfig = tgbot.NewMessage(chatID, "Can not unsubscribe channels.\nInternal server error.")
	} else {
		list := []string{"You already unsubscribe following channels:"}
		set := make(map[int64]bool)

		// Get not repeating channel indices
		for _, i := range elements[1:] {
			idx, err := strconv.ParseInt(i, 10, 64)
			if err != nil || idx >= int64(len(channels)) {
				continue
			}
			set[idx] = true
		}

		// Run unsubscription
		for idx := range set {
			if _, err := s.db.Exec("DELETE FROM subscribers WHERE chatID = ? AND channelID = ?;", chatID, channels[idx].id); err != nil {
				glog.Error(err)
				continue
			}

			chTitle := tgbot.EscapeText(channels[idx].title)
			chLink := tgbot.InlineLink(chTitle, "https://www.youtube.com/channel/"+channels[idx].id)
			list = append(list, chLink)
		}

		// Send message
		msgConfig = tgbot.NewMessage(chatID, strings.Join(list, "\n"))

		// Check not subscribed channels & unsubscribe them from hub
		go func() {
			var chIDs []string

			err := s.db.queryResults(
				&chIDs,
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

			for _, id := range chIDs {
				if _, err := s.db.Exec("DELETE FROM channels WHERE id = ?;", id); err != nil {
					glog.Error(err)
					continue
				}

				s.hub.Unsubscribe(id)
			}
		}()
	}
}

func (s *Server) remindHandler(update tgbot.Update) {
	elements := strings.Fields(update.Message.Text)

	if len(elements) == 1 {
		msgConfig := tgbot.NewMessage(
			update.Message.Chat.ID,
			"Please use "+tgbot.InlineCode("/remind <video url> ...")+" to set video reminder\\.",
		)

		s.tgSend(msgConfig)
		return
	}

	for _, e := range elements[1:] {
		chatID := update.Message.Chat.ID

		if b, err := s.isValidYtVideo(e); err == nil && b {
			_, url, _ := followRedirectURL(e)
			videoID := url.Query()["v"][0]

			if _, err := s.db.Exec(
				"INSERT IGNORE INTO notices (videoID, chatID, messageID) VALUES (?, ?, ?);",
				videoID, chatID, -1,
			); err != nil {
				glog.Error(err)

				msgTemplate := "%s %s failed.\nInternal server error."
				msgTemplate = tgbot.EscapeText(msgTemplate)

				msgConfig := tgbot.NewMessage(chatID, fmt.Sprintf(
					msgTemplate,
					tgbot.ItalicText(tgbot.BordText("Remind")),
					tgbot.InlineLink(videoID, e),
				))

				msgConfig.DisableNotification = true
				msgConfig.DisableWebPagePreview = true

				s.tgSend(msgConfig)
				continue
			}

			entry := hub.Entry{VideoID: videoID}

			go s.noticeHandler(hub.Feed{Entry: &entry})
		}
	}
}

func (s *Server) scheduleHandler(update tgbot.Update) {
	chatID := update.Message.Chat.ID

	var msgConfig tgbot.MessageConfig
	defer func() {
		msgConfig.DisableNotification = true
		msgConfig.DisableWebPagePreview = true
		s.tgSend(msgConfig)
	}()

	var results []struct {
		vID, vTitle string
		chTitle     string
		vStartTime  int64
	}

	err := s.db.queryResults(
		&results,
		func(rows *sql.Rows, dest interface{}) error {
			res := dest.(*struct {
				vID, vTitle string
				chTitle     string
				vStartTime  int64
			})
			return rows.Scan(&res.vID, &res.vTitle, &res.chTitle, &res.vStartTime)
		},
		"SELECT videos.id, videos.title, videos.channelTitle, videos.startTime "+
			"FROM notices INNER JOIN videos ON notices.videoID = videos.id "+
			"WHERE notices.chatID = ?;",
		chatID,
	)

	if err != nil {
		glog.Error(err)
		msgConfig = tgbot.NewMessage(chatID, "Can not list live schedule.\nInternal server error.")
		return
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].vStartTime < results[j].vStartTime
	})

	var list []string

	for _, r := range results {
		if r.vStartTime < time.Now().Unix() {
			continue
		}

		text := fmt.Sprintf(
			"%s %s\n%s",
			time.Unix(r.vStartTime, 0).Local().Format("01/02 15:04"),
			tgbot.ItalicText(tgbot.EscapeText(r.chTitle)),
			tgbot.InlineLink(
				tgbot.BordText(tgbot.EscapeText(r.vTitle)),
				ytVideoURLPrefix+r.vID,
			),
		)

		list = append(list, text)
	}

	if len(list) == 0 {
		list = append(list, "No upcoming live streams\\.")
	}

	msgConfig = tgbot.NewMessage(chatID, strings.Join(list, "\n"))
}

func (s *Server) filterHandler(update tgbot.Update) {
	chatID := update.Message.Chat.ID
	elements := strings.Fields(update.Message.Text)

	var msgConfig tgbot.MessageConfig
	defer func() {
		msgConfig.DisableNotification = true
		msgConfig.DisableWebPagePreview = true
		s.tgSend(msgConfig)
	}()

	if len(elements) == 1 {
		msgConfig = tgbot.NewMessage(
			chatID,
			fmt.Sprintf(
				"Please use `%s` to set filter\\.",
				tgbot.EscapeText("/filter [-show] [-blacklist <word> ...] [-whitelist <word> ...] <channel url>"),
			),
		)
		return
	}

	var show bool = func() bool {
		for i, e := range elements {
			if e == "-show" {
				elements = append(elements[:i], elements[i+1:]...)
				return true
			}
		}
		return false
	}()

	var channel string
	var blacklist, whitelist []string
	var container *[]string

	for i := 1; i < len(elements)-1; i++ {
		switch elements[i] {
		case "-blacklist":
			container = &blacklist
		case "-whitelist":
			container = &whitelist
		default:
			if container == nil {
				channel = elements[i]
			} else {
				*container = append(*container, strings.ToLower(elements[i]))
			}
		}
	}

	if channel == "" {
		channel = elements[len(elements)-1]
	} else {
		*container = append(*container, elements[len(elements)-1])
	}

	if b, err := isValidYtChannel(channel); err == nil && b {
		// If channel is a valid yt channel...
		_, url, _ := followRedirectURL(channel)
		channelID := strings.Split(url.Path, "/")[2]

		var chTitle string
		err := s.db.QueryRow("SELECT title FROM channels WHERE id = ?;", channelID).Scan(&chTitle)
		if err != nil {
			channel = tgbot.EscapeText(channel)
			if err == sql.ErrNoRows {
				msgConfig = tgbot.NewMessage(chatID, fmt.Sprintf("You have not subscribed to %s", channel))
			} else {
				glog.Error(err)
				msgConfig = tgbot.NewMessage(chatID, fmt.Sprintf("Filter setup on %s failed, internal server error", channel))
			}
			return
		} else {
			// If Show only
			if show {
				type rowFilter struct {
					block   bool
					content string
				}

				var err error
				var filters []rowFilter

				err = s.db.queryResults(
					&filters,
					func(rows *sql.Rows, dest interface{}) error {
						filter := dest.(*rowFilter)
						return rows.Scan(&filter.block, &filter.content)
					},
					"SELECT block, content FROM filters WHERE chatID = ? AND channelID = ?;",
					chatID, channelID,
				)

				if err != nil {
					glog.Error(err)
					channel := tgbot.EscapeText(channel)
					msgConfig = tgbot.NewMessage(chatID, fmt.Sprintf("Filter show on %s failed, internal server error", channel))
					return
				}

				var black, white string
				for _, f := range filters {
					if f.block {
						black = f.content
					} else {
						white = f.content
					}
				}

				msgConfig = tgbot.NewMessage(chatID, fmt.Sprintf(
					"%s\n\n_blacklist:_\n%s\n\n_whitelist:_\n%s",
					tgbot.InlineLink(
						tgbot.EscapeText(chTitle),
						tgbot.EscapeText(channel),
					),
					tgbot.EscapeText(black),
					tgbot.EscapeText(white),
				))

				return
			}

			// Regular add filter
			_, err := s.db.Exec(
				"INSERT INTO filters (chatID, channelID, block, content) VALUES(?, ?, ?, ?) "+
					"ON DUPLICATE KEY UPDATE content = VALUES(content);",
				chatID, channelID, true, strings.Join(blacklist, ","),
			)

			if err != nil {
				glog.Error(err)
				channel := tgbot.EscapeText(channel)
				msgConfig = tgbot.NewMessage(chatID, fmt.Sprintf("Filter setup on %s failed, internal server error", channel))
				return
			}

			_, err = s.db.Exec(
				"INSERT INTO filters (chatID, channelID, block, content) VALUES(?, ?, ?, ?) "+
					"ON DUPLICATE KEY UPDATE content = VALUES(content);",
				chatID, channelID, false, strings.Join(whitelist, ","),
			)

			if err != nil {
				glog.Error(err)
				channel := tgbot.EscapeText(channel)
				msgConfig = tgbot.NewMessage(chatID, fmt.Sprintf("Filter setup on %s failed, internal server error", channel))
				return
			}

			msgConfig = tgbot.NewMessage(chatID, fmt.Sprintf(
				"%s\n\n_blacklist:_\n%s\n\n_whitelist:_\n%s",
				tgbot.InlineLink(
					tgbot.EscapeText(chTitle),
					tgbot.EscapeText(channel),
				),
				tgbot.EscapeText(strings.Join(blacklist, ",")),
				tgbot.EscapeText(strings.Join(whitelist, ",")),
			))
		}
	} else if err != nil {
		// If valid check failed...
		glog.Warning(err)
		channel := tgbot.EscapeText(channel)
		msgConfig = tgbot.NewMessage(chatID, fmt.Sprintf("Filter setup on %s failed, internal server error", channel))
	} else if !b {
		// If channel isn't a valid yt channel...
		channel := tgbot.EscapeText(channel)
		msgConfig = tgbot.NewMessage(chatID, fmt.Sprintf("%s is not a valid YouTube channel", channel))
	}
}

func (s *Server) autoRecordHandler(update tgbot.Update) {
	chatID := update.Message.Chat.ID
	elements := strings.Fields(update.Message.Text)

	var msgConfig tgbot.MessageConfig
	defer func() {
		msgConfig.DisableNotification = true
		msgConfig.DisableWebPagePreview = true
		s.tgSend(msgConfig)
	}()

	// Help.
	if len(elements) == 1 {
		msgConfig = tgbot.NewMessage(
			chatID,
			fmt.Sprintf(
				"Please use `%s` to set autorecorder\\.",
				tgbot.EscapeText("~autorecord [-show] [-remove] <channel url> ..."),
			),
		)
		return
	}

	var show, remove bool
	for i, e := range elements {
		switch e {
		case "-show":
			elements = append(elements[:i], elements[i+1:]...)
			show = true
		case "-remove":
			elements = append(elements[:i], elements[i+1:]...)
			remove = true
		}
	}

	// Show all autorecords.
	if show {
		var channels []Channel

		err := s.db.queryResults(
			&channels,
			func(rows *sql.Rows, dest interface{}) error {
				ch := dest.(*Channel)
				return rows.Scan(&ch.id, &ch.title)
			},
			"SELECT channels.id, channels.title "+
				"FROM autorecords INNER JOIN channels "+
				"ON autorecords.channelID = channels.id "+
				"WHERE autorecords.chatID = ?;",
			chatID,
		)

		if err != nil {
			glog.Error(err)
			msgConfig = tgbot.NewMessage(chatID, "Failed to show autorecords, internal server error")
			return
		}

		var channelText []string

		for _, ch := range channels {
			channelText = append(
				channelText,
				tgbot.InlineLink(
					tgbot.EscapeText(ch.title),
					"https://www.youtube.com/channel/"+ch.id,
				),
			)
		}

		msgConfig = tgbot.NewMessage(chatID, fmt.Sprintf(
			"You already set autorecorder on these channels:\n%s",
			strings.Join(channelText, "\n"),
		))
		return
	}

	// Normal action.
	var msgText []string

	for _, channel := range elements[1:] {
		var channelID string
		if b, err := isValidYtChannel(channel); err == nil && b {
			// If e is a valid yt channel...
			_, url, _ := followRedirectURL(channel)
			channelID = strings.Split(url.Path, "/")[2]

			// Check whether user subscribed this channel
			var exists bool
			var chTitle string
			err := s.db.QueryRow(
				"SELECT EXISTS(SELECT * FROM subscribers WHERE chatID = ? AND channelID = ?), title "+
					"FROM channels "+
					"WHERE id = ?",
				chatID, channelID, channelID,
			).Scan(&exists, &chTitle)

			chTitle = tgbot.EscapeText(chTitle)

			if err != nil {
				channel = tgbot.EscapeText(channel)

				if err == sql.ErrNoRows {
					msgText = append(msgText, fmt.Sprintf("You have not subscribed to %s", channel))
				} else {
					glog.Error(err)
					msgText = append(msgText, fmt.Sprintf("Failed to modify autorecorder on %s, internal server error", channel))
				}

				continue
			} else if !exists {
				msgText = append(
					msgText,
					fmt.Sprintf(
						"You have not subscribed to %s",
						tgbot.InlineLink(chTitle, channel),
					),
				)

				continue
			}

			if !remove {
				// Add channel to autorecorder table
				if _, err = s.db.Exec(
					"INSERT IGNORE INTO autorecords (chatID, channelID) VALUES (?, ?);",
					chatID, channelID,
				); err != nil {
					glog.Error(err)
					channel = tgbot.EscapeText(channel)
					msgText = append(msgText, fmt.Sprintf(
						"Failed to modify autorecorder on %s, internal server error",
						tgbot.InlineLink(chTitle, channel),
					))
					continue
				}

				msgText = append(msgText, fmt.Sprintf("Add autorecorder on %s", tgbot.InlineLink(chTitle, channel)))
			} else {
				// Remove channel from autorecorder table
				if _, err = s.db.Exec(
					"DELETE FROM autorecords WHERE chatID = ? AND channelID = ?;",
					chatID, channelID,
				); err != nil {
					glog.Error(err)
					msgText = append(msgText, fmt.Sprintf(
						"Failed to modify autorecorder on %s, internal server error",
						tgbot.InlineLink(chTitle, channel),
					))
					continue
				}

				msgText = append(msgText, fmt.Sprintf("Remove autorecorder on %s", tgbot.InlineLink(chTitle, channel)))
			}
		} else if err != nil {
			// If valid check failed...
			glog.Warning(err)
			channel = tgbot.EscapeText(channel)
			msgText = append(msgText, fmt.Sprintf("Failed to add autorecorder to %s, internal server error", channel))
		} else if !b {
			// If e isn't a valid yt channel...
			channel = tgbot.EscapeText(channel)
			msgText = append(msgText, fmt.Sprintf("%s is not a valid YouTube channel", channel))
		}
	}

	// Emtpy message
	if len(msgText) == 0 {
		msgText = append(msgText, "Empty parameters\\.")
	}

	msgConfig = tgbot.NewMessage(chatID, strings.Join(msgText, "\n"))
}

func (s *Server) downloadHandler(update tgbot.Update) {
	chatID := update.Message.Chat.ID
	elements := strings.Fields(update.Message.Text)

	var msgConfig tgbot.MessageConfig
	var internalServerError tgbot.MessageConfig = tgbot.NewMessage(chatID, "Download request failed, internal server error")

	defer func() {
		if msgConfig != (tgbot.MessageConfig{}) {
			msgConfig.DisableNotification = true
			msgConfig.DisableWebPagePreview = true
			s.tgSend(msgConfig)
		}
	}()

	if r, ok := s.recorderTable[chatID]; ok {
		data := make(map[string]interface{})

		data["url"] = elements[1:]

		resp, err := r.Download(s.CallbackUrl(), data)

		if err != nil {
			if err.(*url.Error).Timeout() {
				msgConfig = tgbot.NewMessage(chatID, "Download request failed, connection timeout")
			} else {
				glog.Error(err)
				msgConfig = internalServerError
			}
		} else if resp.StatusCode != http.StatusOK {
			respBody, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				glog.Error(err)
				msgConfig = internalServerError
				return
			}

			fmt.Println(string(respBody))
			msgConfig = tgbot.NewMessage(
				chatID,
				fmt.Sprintf("Download request failed with status code %d, please check your recorder", resp.StatusCode),
			)
		} else {
			msgConfig = tgbot.NewMessage(chatID, "Download request has been accepted")
		}
	}
}

func (s *Server) callbackHandler(update tgbot.Update) {
	// Basic callback info
	callbackID := update.CallbackQuery.ID
	chatID := update.CallbackQuery.Message.Chat.ID
	msgID := update.CallbackQuery.Message.MessageID

	// Decode callback data
	data := make(map[string]interface{})
	json.Unmarshal([]byte(update.CallbackQuery.Data), &data)

	var cfg tgbot.CallbackConfig

	switch data["type"] {
	case "record":
		videoID := data["videoID"]

		if _, err := s.db.Exec(
			"INSERT IGNORE INTO records (chatID, videoID) VALUES (?, ?);",
			chatID, videoID,
		); err != nil {
			glog.Error(err)
			cfg = tgbot.NewCallback(callbackID, "Internal server error")
			s.tg.AnswerCallbackQuery(cfg)
		} else {
			cfg = tgbot.NewCallback(callbackID, fmt.Sprintf("Add %s recorder", videoID))
			s.tg.AnswerCallbackQuery(cfg)

			cfg := tgbot.NewEditMessageReplyMarkup(chatID, msgID,
				tgbot.InlineKeyboardMarkup{InlineKeyboard: [][]tgbot.InlineKeyboardButton{{}}})
			s.tgSend(cfg)
		}
	case "list":
		channelID, ok := data["channelID"]
		if !ok {
			// Turn page
			page := data["page"]
			markup, err := s.newChannelListMarkUp(chatID, int(page.(float64)))
			if err != nil {
				glog.Error(err)
			} else {
				cfg := tgbot.NewEditMessageReplyMarkup(chatID, msgID, *markup)
				s.tgSend(cfg)
			}
		} else {
			// Subscribed channel operation
			delete(data, "channelID")
			b, _ := json.Marshal(data)
			cancel := tgbot.NewInlineKeyboardButtonData("Cancel", string(b))

			data := make(map[string]interface{})
			data["type"] = "remove"
			data["channelID"] = channelID

			// Get channel title
			var channelTitle string
			err := s.db.QueryRow("SELECT title FROM channels WHERE id = ?;", channelID).Scan(&channelTitle)
			if err != nil {
				glog.Error(err)
				cfg = tgbot.NewCallback(callbackID, "Internal server error")
				s.tg.AnswerCallbackQuery(cfg)
				return
			}

			b, _ = json.Marshal(data)
			remove := tgbot.NewInlineKeyboardButtonData("Remove", string(b))

			row := tgbot.NewInlineKeyboardRow(remove, cancel)
			markup := tgbot.NewInlineKeyboardMarkup(row)

			link := tgbot.InlineLink(tgbot.EscapeText(channelTitle), "https://www.youtube.com/channel/"+channelID.(string))
			cfg := tgbot.NewEditMessageTextAndMarkup(chatID, msgID, fmt.Sprintf("Do you want to remove %s?", link), markup)
			s.tgSend(cfg)
		}
	case "remove":
		channelID := data["channelID"].(string)

		// Get channel title
		var channelTitle string
		err := s.db.QueryRow("SELECT title FROM channels WHERE id = ?;", channelID).Scan(&channelTitle)
		if err != nil {
			glog.Error(err)
			cfg = tgbot.NewCallback(callbackID, "Internal server error")
			s.tg.AnswerCallbackQuery(cfg)
			return
		}

		// Remove subscription
		_, err = s.db.Exec("DELETE FROM subscribers WHERE chatID = ? AND channelID = ?;", chatID, channelID)
		if err != nil {
			glog.Error(err)
			cfg = tgbot.NewCallback(callbackID, "Internal server error")
			s.tg.AnswerCallbackQuery(cfg)
			return
		}

		link := tgbot.InlineLink(tgbot.EscapeText(channelTitle), "https://www.youtube.com/channel/"+channelID)
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
	default:
		glog.Errorf("Invalid callback type: %v", data["type"])
	}
}
