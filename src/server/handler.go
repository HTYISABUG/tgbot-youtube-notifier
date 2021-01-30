package server

import (
	"database/sql"
	"errors"
	"fmt"
	"hub"
	"log"
	"sort"
	"strconv"
	"strings"
	"tgbot"
	"time"
	"ytapi"
)

func (s *Server) subscribeHandler(update tgbot.Update) {
	elements := strings.Fields(update.Message.Text)

	if len(elements) == 1 {
		msgConfig := tgbot.NewMessage(
			update.Message.Chat.ID,
			"Please use `\\/sub <channel url\\> \\.\\.\\.` to subscribe\\.",
		)

		if _, err := s.tg.Send(msgConfig); err != nil {
			log.Println(err)
		}

		return
	}

	chatID := update.Message.Chat.ID

	for _, e := range elements[1:] {
		var msgConfig tgbot.MessageConfig

		if b, err := isValidYtChannel(e); err == nil && b {
			// If e is a valid yt channel...
			_, url, _ := followRedirectURL(e)
			channelID := strings.Split(url.Path, "/")[2]

			// Run subscription & get channel title
			title, err := s.subscribe(rowChat{id: chatID}, rowChannel{id: channelID})
			if title == "" {
				title = channelID
			}

			channelID = tgbot.EscapeText(channelID)
			title = tgbot.EscapeText(title)

			var msgTemplate string
			if err == nil {
				msgTemplate = "%s %s successful."
			} else {
				log.Println(err)
				msgTemplate = "%s %s failed.\n\nIt's a internal server error,\npls contact author or resend later."
			}
			msgTemplate = tgbot.EscapeText(msgTemplate)

			msgConfig = tgbot.NewMessage(chatID, fmt.Sprintf(
				msgTemplate,
				tgbot.ItalicText(tgbot.BordText("Subscribe")),
				tgbot.InlineLink(title, "https://www.youtube.com/channel/"+channelID),
			))
		} else if err != nil {
			// If valid check failed...
			log.Println(err)

			e := tgbot.EscapeText(e)
			msgConfig = tgbot.NewMessage(chatID, fmt.Sprintf("Subscribe %s failed, internal server error", e))
		} else if !b {
			// If e isn't a valid yt channel...
			e := tgbot.EscapeText(e)
			msgConfig = tgbot.NewMessage(chatID, fmt.Sprintf("%s is not a valid YouTube channel", e))
		}

		msgConfig.DisableNotification = true
		msgConfig.DisableWebPagePreview = true

		if _, err := s.tg.Send(msgConfig); err != nil {
			log.Println(err)
		}
	}
}

func (s *Server) subscribe(chat rowChat, channel rowChannel) (string, error) {
	s.hub.Subscribe(channel.id)

	resource, err := s.yt.GetChannelResource(channel.id)
	if err != nil {
		return "", err
	}

	channel.title = resource.Snippet.Title

	if err := s.db.subscribe(chat, channel); err != nil {
		return channel.title, err
	}

	return channel.title, nil
}

func (s *Server) listHandler(update tgbot.Update) {
	chatID := update.Message.Chat.ID

	channels, err := s.db.getSubscribedChannelsByChatID(chatID)
	if err != nil {
		log.Println(err)
	} else {
		list := []string{"You already subscribed following channels:"}

		for i, ch := range channels {
			chID := tgbot.EscapeText(ch.id)
			chTitle := tgbot.EscapeText(ch.title)
			chLink := tgbot.InlineLink(chTitle, "https://www.youtube.com/channel/"+chID)
			list = append(list, fmt.Sprintf("%2d\\|\t%s", i, chLink))
		}

		msgConfig := tgbot.NewMessage(chatID, strings.Join(list, "\n"))
		msgConfig.DisableWebPagePreview = true

		if _, err := s.tg.Send(msgConfig); err != nil {
			log.Println(err)
		}
	}
}

func (s *Server) unsubscribeHandler(update tgbot.Update) {
	chatID := update.Message.Chat.ID
	elements := strings.Fields(update.Message.Text)

	if len(elements) == 1 {
		msgConfig := tgbot.NewMessage(
			chatID,
			"Please use /list to find the channel numbers "+
				"which you want to unsubscribe\\. "+
				"Then use `\\/unsub <number\\> \\.\\.\\.` to unsubscribe\\.",
		)

		if _, err := s.tg.Send(msgConfig); err != nil {
			log.Println(err)
		}
	} else {
		channels, err := s.db.getSubscribedChannelsByChatID(chatID)
		if err != nil {
			log.Println(err)
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
					log.Println(err)
					continue
				}

				chID := tgbot.EscapeText(channels[idx].id)
				chTitle := tgbot.EscapeText(channels[idx].title)
				chLink := tgbot.InlineLink(chTitle, "https://www.youtube.com/channel/"+chID)
				list = append(list, chLink)
			}

			// Check not subscribed channels & unsubscribe them from hub
			go func() {
				rows, err := s.db.Query(
					"SELECT channels.id FROM " +
						"channels LEFT JOIN subscribers ON channels.id = subscribers.channelID " +
						"WHERE subscribers.chatID IS NULL;",
				)
				if err != nil {
					log.Println(err)
					return
				}

				defer rows.Close()

				var cid string
				for rows.Next() {
					if err := rows.Scan(&cid); err != nil {
						log.Println(err)
						continue
					}

					if _, err := s.db.Exec("DELETE FROM channels WHERE id = ?;", cid); err != nil {
						log.Println(err)
						continue
					}

					s.hub.Unsubscribe(cid)
				}

				if rows.Err() != nil {
					log.Println(rows.Err())
					return
				}
			}()

			// Send message
			msgConfig := tgbot.NewMessage(chatID, strings.Join(list, "\n"))
			msgConfig.DisableWebPagePreview = true

			if _, err := s.tg.Send(msgConfig); err != nil {
				log.Println(err)
			}
		}
	}
}

func (s *Server) notifyHandler(feed hub.Feed) {
	if feed.Entry != nil {
		// If it's a normal entry ...

		// Check if it's already exists.
		var exists bool
		err := s.db.QueryRow("SELECT EXISTS(SELECT * FROM videos WHERE id = ?);", feed.Entry.VideoID).Scan(&exists)
		if err != nil && err != sql.ErrNoRows {
			log.Println(err)
			return
		} else if exists {
			// If the video already exists, then check if it's completed.
			var completed bool
			err := s.db.QueryRow("SELECT completed FROM videos WHERE id = ?;", feed.Entry.VideoID).Scan(&completed)
			if err != nil && err != sql.ErrNoRows {
				log.Println(err)
				return
			} else if completed {
				// If the video already completed, then discard.
				return
			}
		}

		// Request corresponding video resource
		resource, err := s.yt.GetVideoResource(
			feed.Entry.VideoID,
			[]string{"snippet", "liveStreamingDetails"},
		)
		if err != nil {
			log.Println(err)
			return
		} else if !resource.IsLiveBroadcast() {
			// If the video is not a live broadcast, then discard.
			// Also record it as completed.
			_, err := s.db.Exec(
				"INSERT INTO videos (id, completed) VALUES (?, ?)"+
					"ON DUPLICATE KEY UPDATE completed = VALUES(completed);",
				resource.ID, true,
			)
			if err != nil {
				log.Println(err)
			}
			return
		}

		s.sendVideoNotify(resource)
		s.tryDiligentScheduler(resource)

		// Update channel title
		_, err = s.db.Exec("UPDATE channels SET title = ? WHERE id = ?;", resource.Snippet.ChannelTitle, resource.Snippet.ChannelID)
		if err != nil {
			log.Println(err)
		}
	} else if feed.DeletedEntry != nil {
		// Get video id
		videoID := strings.Split(feed.DeletedEntry.Ref, ":")[2]

		// Query monitoring rows according to video id.
		mMessages, err := s.db.getMonitoringMessagesByVideoID(videoID)
		if err != nil {
			log.Println(err)
			return
		}

		// Remove monitoring messages.
		for _, msg := range mMessages {
			if msg.messageID != -1 {
				deleteMsgConfig := tgbot.NewDeleteMessage(msg.chatID, msg.messageID)
				_, err := s.tg.DeleteMessage(deleteMsgConfig)
				if err != nil {
					log.Println(err)
				}
			}
		}

		// Remove deleted video from monitoring table.
		if _, err := s.db.Exec("DELETE FROM monitoring WHERE videoID = ?;", videoID); err != nil {
			log.Println(err)
		}
	} else {
		log.Println(errors.New("Receive a empty feed"))
	}
}

func (s *Server) sendVideoNotify(resource ytapi.VideoResource) {
	// Record video infos
	t, _ := time.Parse(time.RFC3339, resource.LiveStreamingDetails.ScheduledStartTime)
	_, err := s.db.Exec(
		"INSERT INTO videos (id, completed, title, startTime, channelID) VALUES (?, ?, ?, ?, ?)"+
			"ON DUPLICATE KEY UPDATE title = VALUES(title), startTime = VALUES(startTime), channelID = VALUES(channelID);",
		resource.ID, false, resource.Snippet.Title, t.Unix(), resource.Snippet.ChannelID,
	)
	if err != nil {
		log.Println(err)
		return
	}

	// Query subscribed chats from db according to channel id.
	chats, err := s.db.getSubscribeChatsByChannelID(resource.Snippet.ChannelID)
	if err != nil {
		log.Println(err)
		return
	}

	// Insert or ignore new rows to monitoring table.
	for _, c := range chats {
		if _, err := s.db.Exec(
			"INSERT IGNORE INTO monitoring (videoID, chatID, messageID) VALUES (?, ?, ?);",
			resource.ID, c.id, -1,
		); err != nil {
			log.Println(err)
		}
	}

	// Query monitoring rows according to video id.
	mMessages, err := s.db.getMonitoringMessagesByVideoID(resource.ID)
	if err != nil {
		log.Println(err)
		return
	}

	for _, mMsg := range mMessages {
		if mMsg.messageID == -1 {
			// If this chat still not being notified, send new notify message.
			msgConfig := tgbot.NewMessage(mMsg.chatID, newNotifyMessageText(resource))
			message, err := s.tg.Send(msgConfig)

			if err != nil {
				log.Println(err)
				fmt.Println(msgConfig.Text)
			} else {
				mMsg.messageID = message.MessageID
				if _, err := s.db.Exec(
					"UPDATE monitoring SET messageID = ? WHERE videoID = ? AND chatID = ?;",
					mMsg.messageID, mMsg.videoID, mMsg.chatID,
				); err != nil {
					log.Println(err)
				}
			}
		} else {
			// If this chat has be notified, edit existing notify message.
			const notModified = "Bad Request: message is not modified"

			editMsgConfig := tgbot.NewEditMessageText(mMsg.chatID, mMsg.messageID, newNotifyMessageText(resource))
			_, err := s.tg.Send(editMsgConfig)
			if err != nil && !strings.HasPrefix(err.Error(), notModified) {
				log.Println(err)
				fmt.Println(editMsgConfig.Text)
			}
		}
	}

	// It's a completed live.
	if resource.IsCompletedLiveBroadcast() {
		// Tag it as completed in videos table.
		_, err := s.db.Exec("UPDATE videos SET completed = ? WHERE id = ?;", true, resource.ID)
		if err != nil {
			log.Println(err)
		}

		// Remove it from monitoring table.
		if _, err := s.db.Exec("DELETE FROM monitoring WHERE videoID = ?;", resource.ID); err != nil {
			log.Println(err)
		}
	}
}

const ytVideoURLPrefix = "https://www.youtube.com/watch?v="

func newNotifyMessageText(resource ytapi.VideoResource) string {
	// Create basic info (title, link, channel).
	basic := fmt.Sprintf(
		"%s\n%s",
		tgbot.InlineLink(
			tgbot.BordText(tgbot.EscapeText(resource.Snippet.Title)),
			ytVideoURLPrefix+tgbot.EscapeText(resource.ID),
		),
		tgbot.ItalicText(tgbot.EscapeText(resource.Snippet.ChannelTitle)),
	)

	liveStreamingDetails := resource.LiveStreamingDetails

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
			"Please use `\\/remind <video url\\> \\.\\.\\.` to set video reminder\\.",
		)

		if _, err := s.tg.Send(msgConfig); err != nil {
			log.Println(err)
		}

		return
	}

	for _, e := range elements[1:] {
		chatID := update.Message.Chat.ID

		if b, err := s.isValidYtVideo(e); err == nil && b {
			_, url, _ := followRedirectURL(e)
			videoID := url.Query()["v"][0]

			if _, err := s.db.Exec(
				"INSERT IGNORE INTO monitoring (videoID, chatID, messageID) VALUES (?, ?, ?);",
				videoID, chatID, -1,
			); err != nil {
				log.Println(err)

				msgTemplate := "%s %s failed.\n\nIt's a internal server error,\npls contact author or resend later."
				msgTemplate = tgbot.EscapeText(msgTemplate)
				videoID = tgbot.EscapeText(videoID)

				msgConfig := tgbot.NewMessage(chatID, fmt.Sprintf(
					msgTemplate,
					tgbot.ItalicText(tgbot.BordText("Remind")),
					tgbot.InlineLink(videoID, "https://www.youtube.com/watch?v="+videoID),
				))

				msgConfig.DisableNotification = true
				msgConfig.DisableWebPagePreview = true

				if _, err := s.tg.Send(msgConfig); err != nil {
					log.Println(err)
				}

				continue
			}

			entry := hub.Entry{VideoID: videoID}

			go s.notifyHandler(hub.Feed{Entry: &entry})
		}
	}
}

func (s *Server) scheduleHandler(update tgbot.Update) {
	chatID := update.Message.Chat.ID

	rows, err := s.db.Query(
		"SELECT videos.id, videos.title, videos.startTime, channels.title "+
			"FROM monitoring INNER JOIN videos ON monitoring.videoID = videos.id "+
			"INNER JOIN channels ON channels.id = videos.channelID "+
			"WHERE monitoring.chatID = ?;",
		chatID,
	)

	if err != nil {
		log.Println(err)
		return
	}

	defer rows.Close()

	var results []struct {
		video   rowVideo
		chTitle string
	}
	var video rowVideo
	var chTitle string
	for rows.Next() {
		err := rows.Scan(&video.id, &video.title, &video.startTime, &chTitle)
		if err != nil {
			log.Println(err)
			continue
		}

		results = append(results, struct {
			video   rowVideo
			chTitle string
		}{video, chTitle})
	}

	if rows.Err() != nil {
		log.Println(rows.Err())
		return
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].video.startTime < results[j].video.startTime
	})

	var list []string

	for _, v := range results {
		if v.video.startTime < time.Now().Unix() {
			continue
		}

		text := fmt.Sprintf(
			"%s %s\n%s",
			time.Unix(v.video.startTime, 0).Local().Format("2006/01/02 15:04:05"),
			tgbot.ItalicText(tgbot.EscapeText(v.chTitle)),
			tgbot.InlineLink(
				tgbot.BordText(tgbot.EscapeText(v.video.title)),
				ytVideoURLPrefix+tgbot.EscapeText(v.video.id),
			),
		)

		list = append(list, text)
	}

	if len(list) == 0 {
		list = append(list, tgbot.EscapeText("No upcoming live streams."))
	}

	msgConfig := tgbot.NewMessage(chatID, strings.Join(list, "\n"))
	msgConfig.DisableNotification = true
	msgConfig.DisableWebPagePreview = true

	_, err = s.tg.Send(msgConfig)
	if err != nil {
		log.Println(err)
		fmt.Println(msgConfig.Text)
	}
}
