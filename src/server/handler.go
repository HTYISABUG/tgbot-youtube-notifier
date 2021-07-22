package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/HTYISABUG/tgbot-youtube-notifier/src/hub"
	"github.com/HTYISABUG/tgbot-youtube-notifier/src/tgbot"
	"github.com/HTYISABUG/tgbot-youtube-notifier/src/ytapi"
	"github.com/golang/glog"
	"google.golang.org/api/youtube/v3"
)

func (s *Server) noticeHandler(feed hub.Feed) {
	if feed.Entry != nil {
		// If it's a normal entry
		// Check if it's already exists.
		var exists bool
		err := s.db.QueryRow("SELECT EXISTS(SELECT * FROM videos WHERE id = ?);", feed.Entry.VideoID).Scan(&exists)
		if err != nil && err != sql.ErrNoRows {
			glog.Error(err)
			return
		} else if exists {
			// If the video already exists, then check if it's completed.
			var completed bool
			err := s.db.QueryRow("SELECT completed FROM videos WHERE id = ?;", feed.Entry.VideoID).Scan(&completed)
			if err != nil && err != sql.ErrNoRows {
				glog.Error(err)
				return
			} else if completed {
				// If the video already completed, then discard.
				return
			}
		}

		// Request corresponding video resource
		v, err := s.yt.GetVideo(
			feed.Entry.VideoID,
			[]string{"snippet", "liveStreamingDetails"},
		)
		if err != nil {
			glog.Warning(err)
			return
		} else if !ytapi.IsLiveBroadcast(v) {
			// If the video is not a live broadcast, then discard.
			// Also record it as completed.
			_, err := s.db.Exec(
				"INSERT INTO videos (id, completed) VALUES (?, ?)"+
					"ON DUPLICATE KEY UPDATE completed = VALUES(completed);",
				v.Id, true,
			)
			if err != nil {
				glog.Error(err)
			}
			return
		}

		// Insert video infos
		t, _ := time.Parse(time.RFC3339, v.LiveStreamingDetails.ScheduledStartTime)
		_, err = s.db.Exec(
			"INSERT INTO videos (id, title, channelID, channelTitle, startTime, completed) VALUES (?, ?, ?, ?, ?, ?)"+
				"ON DUPLICATE KEY UPDATE title = VALUES(title), channelID = VALUES(channelID), "+
				"channelTitle = VALUES(channelTitle), startTime = VALUES(startTime);",
			v.Id, v.Snippet.Title, v.Snippet.ChannelId, v.Snippet.ChannelTitle, t.Unix(), false,
		)
		if err != nil {
			glog.Error(err)
			return
		}

		s.sendVideoNotify(v)
		s.setupVideoAutoRecorder(v)
		s.tryDiligentScheduler(v)

		// Update channel title
		_, err = s.db.Exec("UPDATE channels SET title = ? WHERE id = ?;", v.Snippet.ChannelTitle, v.Snippet.ChannelId)
		if err != nil {
			glog.Error(err)
		}
	} else if feed.DeletedEntry != nil {
		// Get video id
		videoID := strings.Split(feed.DeletedEntry.Ref, ":")[2]

		// Query notice rows according to video id.
		notices, err := s.db.getNoticesByVideoID(videoID)
		if err != nil {
			glog.Error(err)
			return
		}

		// Remove notices.
		for _, n := range notices {
			if n.messageID != -1 {
				deleteMsgConfig := tgbot.NewDeleteMessage(n.chatID, n.messageID)
				_, err := s.tg.DeleteMessage(deleteMsgConfig)
				if err != nil {
					switch err.(type) {
					case tgbot.Error:
						glog.Error(err)
					default:
						glog.Warning(err)
					}
				}
			}
		}

		// Remove deleted video from notices table.
		if _, err := s.db.Exec("DELETE FROM notices WHERE videoID = ?;", videoID); err != nil {
			glog.Error(err)
		}

		// Remove deleted video from records table.
		if _, err := s.db.Exec("DELETE FROM records WHERE videoID = ?;", videoID); err != nil {
			glog.Error(err)
		}
	} else {
		glog.Warning(errors.New("receive a empty feed"))
	}
}

func (s *Server) sendVideoNotify(video *ytapi.Video) {
	// Query subscribed chats from db according to channel id.
	var chIDs []int64

	err := s.db.queryResults(
		&chIDs,
		func(rows *sql.Rows, dest interface{}) error {
			r := dest.(*int64)
			return rows.Scan(r)
		},
		"SELECT chats.id FROM "+
			"chats INNER JOIN subscribers ON chats.id = subscribers.chatID "+
			"WHERE subscribers.channelID = ?;",
		video.Snippet.ChannelId,
	)

	if err != nil {
		glog.Error(err)
		return
	}

	// Insert or ignore new rows to notices table.
	for _, cid := range chIDs {
		b, err := s.applyFilters(cid, video)
		if err != nil {
			glog.Error(err)
			continue
		} else if !b { // No pass
			continue
		}

		if _, err := s.db.Exec(
			"INSERT IGNORE INTO notices (videoID, chatID, messageID) VALUES (?, ?, ?);",
			video.Id, cid, -1,
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

	for _, n := range notices {
		if n.messageID == -1 {
			// If this chat still not being notified, send new notify message.
			msgConfig := tgbot.NewMessage(n.chatID, newNotifyMessageText(video))

			cond, err := s.isRecordButtonShowCondition(n.chatID, video)
			if err != nil {
				glog.Error(err)
			} else if cond {
				button := tgbot.NewInlineKeyboardButtonData("Record", video.Id)
				row := tgbot.NewInlineKeyboardRow(button)
				markup := tgbot.NewInlineKeyboardMarkup(row)
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
			// If this chat has be notified, edit existing notify message.
			editMsgConfig := tgbot.NewEditMessageText(n.chatID, n.messageID, newNotifyMessageText(video))

			cond, err := s.isRecordButtonShowCondition(n.chatID, video)
			if err != nil {
				glog.Error(err)
			} else if cond {
				button := tgbot.NewInlineKeyboardButtonData("Record", video.Id)
				row := tgbot.NewInlineKeyboardRow(button)
				markup := tgbot.NewInlineKeyboardMarkup(row)
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

func (s *Server) isRecordButtonShowCondition(chatID int64, video *youtube.Video) (bool, error) {
	// Check user recorder existence
	var exist bool
	err := s.db.QueryRow(
		"SELECT EXISTS(SELECT * FROM chats WHERE id = ? AND recorder IS NOT NULL AND token IS NOT NULL);",
		chatID,
	).Scan(&exist)

	if err != nil {
		return false, err
	} else if !exist {
		return false, nil
	}

	// Check autorecorder existence
	err = s.db.QueryRow(
		"SELECT EXISTS(SELECT * FROM autorecords WHERE chatID = ? AND channelID = ?);",
		chatID, video.Snippet.ChannelId,
	).Scan(&exist)

	if err != nil {
		return false, err
	} else if exist {
		return false, nil
	}

	// Check recorder existence
	err = s.db.QueryRow(
		"SELECT EXISTS(SELECT * FROM records WHERE chatID = ? AND videoID = ?);",
		chatID, video.Id,
	).Scan(&exist)

	if err != nil {
		return false, err
	} else if exist {
		return false, nil
	}

	return true, nil
}

func (s *Server) setupVideoAutoRecorder(video *ytapi.Video) {
	// Query autorecorders from db according to channel id.
	var chatIDs []int64

	err := s.db.queryResults(
		&chatIDs,
		func(rows *sql.Rows, dest interface{}) error {
			r := dest.(*int64)
			return rows.Scan(r)
		},
		"SELECT autorecords.chatID FROM autorecords WHERE channelID = ?;",
		video.Snippet.ChannelId,
	)

	if err != nil {
		glog.Error(err)
		return
	}

	// Insert or ignore new rows to records table.
	for _, cid := range chatIDs {
		b, err := s.applyFilters(cid, video)
		if err != nil {
			glog.Error(err)
			continue
		} else if !b { // No pass
			continue
		}

		if _, err := s.db.Exec(
			"INSERT IGNORE INTO records (chatID, videoID) VALUES (?, ?);",
			cid, video.Id,
		); err != nil {
			glog.Error(err)
			continue
		}
	}
}

func (s *Server) applyFilters(chatID int64, video *ytapi.Video) (bool, error) {
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
		chatID, video.Snippet.ChannelId,
	)

	if err != nil {
		return false, err
	}

	title := strings.ToLower(video.Snippet.Title)

	for _, f := range filters {
		if f.content != "" {
			words := strings.Split(f.content, ",")

			if f.block == containsAny(title, words) {
				glog.Infof("Apply filter {chatID: %v\tblock: %v\tcontent:%v}", chatID, f.block, f.content)
				return false, nil
			}
		}
	}

	return true, nil
}

const ytVideoURLPrefix = "https://www.youtube.com/watch?v="

func newNotifyMessageText(video *ytapi.Video) string {
	// Create basic info (title, link, channel).
	basic := fmt.Sprintf(
		"%s\n%s",
		tgbot.InlineLink(
			tgbot.BordText(tgbot.EscapeText(video.Snippet.Title)),
			ytVideoURLPrefix+video.Id,
		),
		tgbot.ItalicText(tgbot.EscapeText(video.Snippet.ChannelTitle)),
	)

	liveStreamingDetails := video.LiveStreamingDetails

	if liveStreamingDetails == nil {
		return basic
	}

	scheduledStartTime := liveStreamingDetails.ScheduledStartTime
	actualStartTime := liveStreamingDetails.ActualStartTime
	actualEndTime := liveStreamingDetails.ActualEndTime

	var t time.Time
	var liveStatus string
	var timeTitle, timeDetail string
	var appendix string

	if actualEndTime != "" {
		// It's a completed live.
		liveStatus = "Completed"

		timeTitle = "Actual End Time"
		t, _ = time.Parse(time.RFC3339, actualEndTime)
		timeDetail = t.Local().Format("2006/01/02 15:04:05")

		start, _ := time.Parse(time.RFC3339, actualStartTime)
		dur := t.Sub(start).Round(time.Second)
		appendix = fmt.Sprintf(
			"%02d:%02d:%02d",
			int(dur.Hours()),
			int(dur.Minutes())%60,
			int(dur.Seconds())%60,
		)
	} else if actualStartTime != "" {
		// It's a live live.
		liveStatus = "Live"

		timeTitle = "Actual Start Time"
		t, _ = time.Parse(time.RFC3339, actualStartTime)
		timeDetail = t.Local().Format("2006/01/02 15:04:05")
	} else if scheduledStartTime != "" {
		// It's a upcoming live.
		liveStatus = "Upcoming"

		timeTitle = "Scheduled Start Time"
		t, _ = time.Parse(time.RFC3339, scheduledStartTime)
		timeDetail = t.Local().Format("2006/01/02 15:04:05")
	}

	detail := fmt.Sprintf(
		"%s\n%s\n\n%s\n%s",
		tgbot.BordText("Status"),
		tgbot.ItalicText(liveStatus),
		tgbot.BordText(timeTitle),
		tgbot.ItalicText(timeDetail),
	)

	if appendix != "" {
		detail = fmt.Sprintf(
			"%s\n\n%s\n%s",
			detail,
			tgbot.BordText("Duration"),
			tgbot.ItalicText(appendix),
		)
	}

	return fmt.Sprintf("%s\n\n%s", basic, detail)
}

// recorderHandler handle completed notify request from recorder
func (s *Server) recorderHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	body, _ := ioutil.ReadAll(r.Body)

	var data struct {
		Action string `json:"action"`
	}

	_ = json.Unmarshal(body, &data)

	switch data.Action {
	case "record":
		s.recorderRecordHandler(w, r, body)
	case "download":
		s.recorderDownloadHandler(w, r, body)
	default:
		glog.Error("Invalid action type:", data.Action)
	}
}

func (s *Server) recorderRecordHandler(w http.ResponseWriter, r *http.Request, body []byte) {
	var data struct {
		Success  bool   `json:"success"`
		ChatID   int64  `json:"chatID"`
		VideoID  string `json:"videoID"`
		Filename string `json:"filename"`
	}

	_ = json.Unmarshal(body, &data)

	v, err := s.yt.GetVideo(
		data.VideoID,
		[]string{"snippet", "liveStreamingDetails"},
	)
	if err != nil {
		glog.Warning(err)
		return
	} else if !data.Success {
		msgConfig := tgbot.NewMessage(
			data.ChatID,
			fmt.Sprintf(
				"Failed to record %s, check your recorder",
				tgbot.InlineLink(tgbot.EscapeText(v.Snippet.Title), ytVideoURLPrefix+v.Id),
			),
		)
		msgConfig.DisableNotification = true

		s.tgSend(msgConfig)
		return
	} else if data.Success && ytapi.IsLiveBroadcast(v) && !ytapi.IsCompletedLiveBroadcast(v) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			Retry bool `json:"retry"`
		}{Retry: true})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		Retry bool `json:"retry"`
	}{Retry: false})

	msgConfig := tgbot.NewMessage(
		data.ChatID,
		fmt.Sprintf(
			"%s recorded as\n%s",
			tgbot.InlineLink(tgbot.EscapeText(v.Snippet.Title), ytVideoURLPrefix+v.Id),
			tgbot.InlineCode(tgbot.EscapeText(data.Filename)),
		),
	)
	msgConfig.DisableNotification = true
	msgConfig.DisableWebPagePreview = true

	s.tgSend(msgConfig)

	// Remove record from table
	if _, err = s.db.Exec(
		"DELETE FROM records WHERE chatID = ? AND videoID = ?;",
		data.ChatID, data.VideoID,
	); err != nil {
		glog.Error(err)
		return
	}
}

func (s *Server) recorderDownloadHandler(w http.ResponseWriter, r *http.Request, body []byte) {
	var data struct {
		Success     bool   `json:"success"`
		Description string `json:"description"`
		ChatID      int64  `json:"chatID"`
		VideoID     string `json:"videoID"`
		Filename    string `json:"filename"`
	}

	_ = json.Unmarshal(body, &data)

	if !data.Success {
		msgConfig := tgbot.NewMessage(
			data.ChatID,
			tgbot.EscapeText(data.Description),
		)
		msgConfig.DisableNotification = true
		msgConfig.DisableWebPagePreview = true

		s.tgSend(msgConfig)
		return
	}

	extRemoved := data.Filename[:strings.LastIndex(data.Filename, ".")]
	title := extRemoved[:strings.LastIndex(extRemoved, ".")]

	msgConfig := tgbot.NewMessage(
		data.ChatID,
		fmt.Sprintf(
			"%s downloaded as\n%s",
			tgbot.InlineLink(tgbot.EscapeText(title), ytVideoURLPrefix+data.VideoID),
			tgbot.InlineCode(tgbot.EscapeText(data.Filename)),
		),
	)
	msgConfig.DisableNotification = true
	msgConfig.DisableWebPagePreview = true

	s.tgSend(msgConfig)
}

func (s *Server) callbackHandler(update tgbot.Update) {
	callbackID := update.CallbackQuery.ID
	chatID := update.CallbackQuery.Message.Chat.ID
	msgID := update.CallbackQuery.Message.MessageID
	videoID := update.CallbackQuery.Data

	var cfg tgbot.CallbackConfig

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
}
