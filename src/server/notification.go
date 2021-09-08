package server

import (
	"encoding/json"
	"fmt"

	"github.com/HTYISABUG/tgbot-youtube-notifier/src/tgbot"
	"github.com/HTYISABUG/tgbot-youtube-notifier/src/ytapi"
	"github.com/golang/glog"
	"google.golang.org/api/youtube/v3"
)

func (s *Server) sendNotices(video *ytapi.Video) {
	// Query chats that subscribed channel.
	chats, err := s.db.getChatsByChannelID(video.Snippet.ChannelId)
	if err != nil {
		glog.Error(err)
		return
	}

	// Insert or ignore new rows to notices table.
	for _, c := range chats {
		b, err := s.applyFilters(c.id, video)
		if err != nil {
			glog.Error(err)
			continue
		} else if !b { // Filter out
			continue
		}

		if _, err := s.db.Exec(
			"INSERT IGNORE INTO notices (videoID, chatID, messageID) VALUES (?, ?, ?);",
			video.Id, c.id, -1,
		); err != nil {
			glog.Error(err)
		}
	}

	// Query notice rows according to video id.
	notices, err := s.db.getNoticesByVideoID(video.Id)
	if err != nil {
		glog.Error(err)
		return
	}

	// Send notices
	for _, n := range notices {
		show, err := s.showRecordButton(n.chatID, video)
		if err != nil {
			glog.Error(err)
		}

		if n.messageID == -1 {
			// If this chat still not being notified, send new notice.
			msgConfig := tgbot.NewMessage(n.chatID, newNotifyMessageText(video))

			if show {
				markup := newRecordButton(video.Id)
				msgConfig.ReplyMarkup = markup
			}

			// Send message
			message, err := s.tg.Send(msgConfig)
			if err != nil {
				switch err.(type) {
				case tgbot.Error:
					glog.Error(err)
					fmt.Println(msgConfig.Text)
				default:
					glog.Warning(err)
				}
			}

			n.messageID = message.MessageID
			if _, err := s.db.Exec(
				"UPDATE notices SET messageID = ? WHERE videoID = ? AND chatID = ?;",
				n.messageID, n.videoID, n.chatID,
			); err != nil {
				glog.Error(err)
			}
		} else {
			// If this chat has be notified, edit existing notice.
			editMsgConfig := tgbot.NewEditMessageText(n.chatID, n.messageID, newNotifyMessageText(video))

			if show {
				markup := newRecordButton(video.Id)
				editMsgConfig.ReplyMarkup = &markup
			}

			s.tgSend(editMsgConfig)
		}
	}

	// It's a completed live.
	if ytapi.IsCompletedLiveBroadcast(video) {
		// Tag it as completed in videos table.
		_, err := s.db.Exec("UPDATE videos SET completed = ? WHERE id = ?;", true, video.Id)
		if err != nil {
			glog.Error(err)
		}

		// Remove it from notices table.
		if _, err := s.db.Exec("DELETE FROM notices WHERE videoID = ?;", video.Id); err != nil {
			glog.Error(err)
		}
	}
}

func (s *Server) showRecordButton(chatID int64, video *youtube.Video) (bool, error) {
	recordable, err := s.isRecordableChat(chatID)
	if err != nil {
		return false, err
	}

	autoExist, err := s.isAutoRecorderExist(chatID, video.Snippet.ChannelId)
	if err != nil {
		return false, err
	}

	recordExist, err := s.isRecordExist(chatID, video.Id)
	if err != nil {
		return false, err
	}

	return recordable && !autoExist && !recordExist, nil
}

func (s *Server) isRecordableChat(chatID int64) (bool, error) {
	var exist bool
	err := s.db.QueryRow(
		"SELECT EXISTS(SELECT * FROM chats WHERE id = ? AND recorder IS NOT NULL AND token IS NOT NULL);",
		chatID,
	).Scan(&exist)

	if err != nil {
		return false, err
	} else {
		return exist, nil
	}
}

func (s *Server) isAutoRecorderExist(chatID int64, channelID string) (bool, error) {
	var exist bool
	err := s.db.QueryRow(
		"SELECT EXISTS(SELECT * FROM autorecords WHERE chatID = ? AND channelID = ?);",
		chatID, channelID,
	).Scan(&exist)

	if err != nil {
		return false, err
	} else {
		return exist, nil
	}
}

func (s *Server) isRecordExist(chatID int64, videoID string) (bool, error) {
	var exist bool
	err := s.db.QueryRow(
		"SELECT EXISTS(SELECT * FROM records WHERE chatID = ? AND videoID = ?);",
		chatID, videoID,
	).Scan(&exist)

	if err != nil {
		return false, err
	} else {
		return exist, nil
	}
}

func newRecordButton(videoID string) tgbot.InlineKeyboardMarkup {
	data := make(map[string]interface{})
	data["type"] = "record"
	data["videoID"] = videoID
	b, _ := json.Marshal(data)

	button := tgbot.NewInlineKeyboardButtonData("Record", string(b))
	row := tgbot.NewInlineKeyboardRow(button)
	markup := tgbot.NewInlineKeyboardMarkup(row)
	return markup
}
