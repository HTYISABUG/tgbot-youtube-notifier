package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
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
					glog.Warningln(err)
					msgTemplate = "%s %s failed.\nInternal server error."
				}
			} else {
				title = c.Snippet.Title
				// Insert into database.
				if err := s.db.subscribe(rowChat{id: chatID}, rowChannel{id: c.Id, title: c.Snippet.Title}); err != nil {
					glog.Warningln(err)
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
			glog.Warningln(err)
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

	channels, err := s.db.getChannelsByChatID(chatID)
	if err != nil {
		glog.Errorln(err)
		msgConfig = tgbot.NewMessage(chatID, "Can not list subscribed channels.\nInternal server error.")
	} else {
		list := []string{"You already subscribed following channels:"}

		for i, ch := range channels {
			chLink := tgbot.InlineLink(
				tgbot.EscapeText(ch.title),
				"https://www.youtube.com/channel/"+ch.id,
			)
			list = append(list, fmt.Sprintf("%2d\\|\t%s", i, chLink))
		}

		msgConfig = tgbot.NewMessage(chatID, strings.Join(list, "\n"))
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
		glog.Errorln(err)
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
				glog.Errorln(err)
				continue
			}

			chTitle := tgbot.EscapeText(channels[idx].title)
			chLink := tgbot.InlineLink(chTitle, "https://www.youtube.com/channel/"+channels[idx].id)
			list = append(list, chLink)
		}

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
				glog.Errorln(err)
				return
			}

			for _, id := range chIDs {
				if _, err := s.db.Exec("DELETE FROM channels WHERE id = ?;", id); err != nil {
					glog.Errorln(err)
					continue
				}

				s.hub.Unsubscribe(id)
			}
		}()

		// Send message
		msgConfig = tgbot.NewMessage(chatID, strings.Join(list, "\n"))
	}
}

func (s *Server) noticeHandler(feed hub.Feed) {
	if feed.Entry != nil {
		// If it's a normal entry
		// Check if it's already exists.
		var exists bool
		err := s.db.QueryRow("SELECT EXISTS(SELECT * FROM videos WHERE id = ?);", feed.Entry.VideoID).Scan(&exists)
		if err != nil && err != sql.ErrNoRows {
			glog.Errorln(err)
			return
		} else if exists {
			// If the video already exists, then check if it's completed.
			var completed bool
			err := s.db.QueryRow("SELECT completed FROM videos WHERE id = ?;", feed.Entry.VideoID).Scan(&completed)
			if err != nil && err != sql.ErrNoRows {
				glog.Errorln(err)
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
			glog.Warningln(err)
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
				glog.Errorln(err)
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
			glog.Errorln(err)
			return
		}

		s.sendVideoNotify(v)
		s.setupVideoRecorder(v)
		s.tryDiligentScheduler(v)

		// Update channel title
		_, err = s.db.Exec("UPDATE channels SET title = ? WHERE id = ?;", v.Snippet.ChannelTitle, v.Snippet.ChannelId)
		if err != nil {
			glog.Errorln(err)
		}
	} else if feed.DeletedEntry != nil {
		// Get video id
		videoID := strings.Split(feed.DeletedEntry.Ref, ":")[2]

		// Query notice rows according to video id.
		notices, err := s.db.getNoticesByVideoID(videoID)
		if err != nil {
			glog.Errorln(err)
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
						glog.Errorln(err)
					default:
						glog.Warningln(err)
					}
				}
			}
		}

		// Remove deleted video from notices table.
		if _, err := s.db.Exec("DELETE FROM notices WHERE videoID = ?;", videoID); err != nil {
			glog.Errorln(err)
		}
	} else {
		glog.Warningln(errors.New("receive a empty feed"))
	}
}

func (s *Server) sendVideoNotify(video *ytapi.Video) {
	// Query subscribed chats from db according to channel id.
	var chats []rowChat

	err := s.db.queryResults(
		&chats,
		func(rows *sql.Rows, dest interface{}) error {
			r := dest.(*rowChat)
			return rows.Scan(&r.id)
		},
		"SELECT chats.id FROM "+
			"chats INNER JOIN subscribers ON chats.id = subscribers.chatID "+
			"WHERE subscribers.channelID = ?;",
		video.Snippet.ChannelId,
	)

	if err != nil {
		glog.Errorln(err)
		return
	}

	// Insert or ignore new rows to notices table.
	for _, c := range chats {
		b, err := s.applyFilters(c.id, video)
		if err != nil {
			glog.Errorln(err)
			continue
		} else if !b { // No pass
			continue
		}

		if _, err := s.db.Exec(
			"INSERT IGNORE INTO notices (videoID, chatID, messageID) VALUES (?, ?, ?);",
			video.Id, c.id, -1,
		); err != nil {
			glog.Errorln(err)
		}
	}

	// Query notice rows according to video id.
	notices, err := s.db.getNoticesByVideoID(video.Id)
	if err != nil {
		glog.Errorln(err)
		return
	}

	for _, n := range notices {
		if n.messageID == -1 {
			// If this chat still not being notified, send new notify message.
			msgConfig := tgbot.NewMessage(n.chatID, newNotifyMessageText(video))
			message, err := s.tg.Send(msgConfig)
			if err != nil {
				switch err.(type) {
				case tgbot.Error:
					glog.Errorln(err)
					fmt.Println(msgConfig.Text)
				default:
					glog.Warningln(err)
				}
			}

			n.messageID = message.MessageID
			if _, err := s.db.Exec(
				"UPDATE notices SET messageID = ? WHERE videoID = ? AND chatID = ?;",
				n.messageID, n.videoID, n.chatID,
			); err != nil {
				glog.Errorln(err)
			}
		} else {
			// If this chat has be notified, edit existing notify message.
			editMsgConfig := tgbot.NewEditMessageText(n.chatID, n.messageID, newNotifyMessageText(video))

			s.tgSend(editMsgConfig)
		}
	}

	// It's a completed live.
	if ytapi.IsCompletedLiveBroadcast(video) {
		// Tag it as completed in videos table.
		_, err := s.db.Exec("UPDATE videos SET completed = ? WHERE id = ?;", true, video.Id)
		if err != nil {
			glog.Errorln(err)
		}

		// Remove it from notices table.
		if _, err := s.db.Exec("DELETE FROM notices WHERE videoID = ?;", video.Id); err != nil {
			glog.Errorln(err)
		}
	}
}

func (s *Server) setupVideoRecorder(video *ytapi.Video) {
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
		glog.Errorln(err)
		return
	}

	// Insert or ignore new rows to records table.
	for _, cid := range chatIDs {
		b, err := s.applyFilters(cid, video)
		if err != nil {
			glog.Errorln(err)
			continue
		} else if !b { // No pass
			continue
		}

		if _, err := s.db.Exec(
			"INSERT IGNORE INTO records (chatID, videoID) VALUES (?, ?);",
			cid, video.Id,
		); err != nil {
			glog.Errorln(err)
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
				glog.Errorln(err)

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
		glog.Errorln(err)
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
				glog.Errorln(err)
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
					glog.Errorln(err)
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
				glog.Errorln(err)
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
				glog.Errorln(err)
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
		glog.Warningln(err)
		channel := tgbot.EscapeText(channel)
		msgConfig = tgbot.NewMessage(chatID, fmt.Sprintf("Filter setup on %s failed, internal server error", channel))
	} else if !b {
		// If channel isn't a valid yt channel...
		channel := tgbot.EscapeText(channel)
		msgConfig = tgbot.NewMessage(chatID, fmt.Sprintf("%s is not a valid YouTube channel", channel))
	}
}

func (s *Server) tgSend(c tgbot.Chattable) {
	_, err := s.tg.Send(c)
	if err != nil {
		switch err.(type) {
		case tgbot.Error:
			switch cfg := c.(type) {
			case tgbot.MessageConfig:
				glog.Errorln(err)
				fmt.Println(cfg.Text)
				debug.PrintStack()
			case tgbot.EditMessageTextConfig:
				const notModified = "Bad Request: message is not modified"

				if !strings.HasPrefix(err.Error(), notModified) {
					glog.Errorln(err)
					fmt.Println(cfg.Text)
					debug.PrintStack()
				}
			default:
				fmt.Printf("%+v\n", cfg)
				debug.PrintStack()
			}
		default:
			glog.Warningln(err)
		}
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
		var channels []rowChannel

		err := s.db.queryResults(
			&channels,
			func(rows *sql.Rows, dest interface{}) error {
				ch := dest.(*rowChannel)
				return rows.Scan(&ch.id, &ch.title)
			},
			"SELECT channels.id, channels.title "+
				"FROM autorecords INNER JOIN channels "+
				"ON autorecords.channelID = channels.id "+
				"WHERE autorecords.chatID = ?;",
			chatID,
		)

		if err != nil {
			glog.Errorln(err)
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
					glog.Errorln(err)
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
					glog.Errorln(err)
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
					glog.Errorln(err)
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
			glog.Warningln(err)
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

// recorderHandler handle completed notify request from recorder
func (s *Server) recorderHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	body, _ := ioutil.ReadAll(r.Body)

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
		glog.Warningln(err)
		return
	} else if !data.Success {
		msgConfig := tgbot.NewMessage(
			data.ChatID,
			fmt.Sprintf(
				"Failed to record %s, check your recorder",
				tgbot.InlineLink(v.Snippet.Title, ytVideoURLPrefix+v.Id),
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
		glog.Errorln(err)
		return
	}
}
