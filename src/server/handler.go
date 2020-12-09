package server

import (
	"database/sql"
	"errors"
	"fmt"
	"hub"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"tgbot"
	"time"
	"ytapi"
)

func (s *Server) subscribeHandler(update tgbot.Update) {
	elements := strings.Fields(update.Message.Text)

	for _, e := range elements[1:] {
		userID := update.Message.From.ID
		chatID := update.Message.Chat.ID

		var msgConfig tgbot.MessageConfig

		if b, err := s.isValidYtChannel(e); err == nil && b {
			// If e is a valid yt channel...
			url, _ := url.Parse(e)
			channelID := strings.Split(url.Path, "/")[2]

			// Run subscription & get channel title
			title, err := s.subscribe(rowUser{userID, chatID}, rowChannel{id: channelID})
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

func (s *Server) isValidYtChannel(rawurl string) (bool, error) {
	const (
		ytHost     = "youtube.com"
		ytHostFull = "www.youtube.com"
	)

	url, err := url.Parse(rawurl)
	if err != nil {
		return false, nil
	}

	if url.Scheme == "" {
		url.Scheme = "https"
		url, _ = url.Parse(url.String())
	}

	if url.Scheme == "http" || url.Scheme == "https" &&
		(url.Host == ytHost || url.Host == ytHostFull) &&
		strings.HasPrefix(url.Path, "/channel") {
		resp, err := http.Get(url.String())

		if err != nil {
			return false, err
		} else if resp.StatusCode == http.StatusOK {
			return true, nil
		}
	}

	return false, nil
}

func (s *Server) subscribe(user rowUser, channel rowChannel) (string, error) {
	s.hub.Subscribe(channel.id)

	resource, err := s.yt.GetChannelResource(channel.id)
	if err != nil {
		return "", err
	}

	channel.title = resource.Snippet.Title

	if err := s.db.subscribe(user, channel); err != nil {
		return channel.title, err
	}

	return channel.title, nil
}

func (s *Server) listHandler(update tgbot.Update) {
	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID

	channels, err := s.db.getSubscribedChannelsByUserID(userID)
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
	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID
	elements := strings.Fields(update.Message.Text)

	if len(elements) == 1 {
		msgConfig := tgbot.NewMessage(
			chatID,
			"Please use /list to find the channel numbers "+
				"which you want to unsubscribe\\. "+
				"Then use `\\/unsubscribe <number\\> \\.\\.\\.` to unsubscribe\\.",
		)

		if _, err := s.tg.Send(msgConfig); err != nil {
			log.Println(err)
		}
	} else {
		channels, err := s.db.getSubscribedChannelsByUserID(userID)
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
				if _, err := s.db.Exec("DELETE FROM subscribers WHERE userID = ? AND channelID = ?;", userID, channels[idx].id); err != nil {
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
						"WHERE subscribers.userID IS NULL;",
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
		} else {
			_, err := s.db.Exec(
				"INSERT IGNORE INTO videos (id, completed) VALUES (?, ?);",
				resource.ID, false,
			)
			if err != nil {
				log.Println(err)
			}
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
	// Query subscribed users from db according to channel id.
	users, err := s.db.getSubscribeUsersByChannelID(resource.Snippet.ChannelID)
	if err != nil {
		log.Println(err)
		return
	}

	// Insert or ignore new rows to monitoring table.
	for _, u := range users {
		if _, err := s.db.Exec(
			"INSERT IGNORE INTO monitoring (videoID, chatID, messageID) VALUES (?, ?, ?);",
			resource.ID, u.chatID, -1,
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
			// If this user still not being notified, send new notify message.
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
			// If this user has be notified, edit existing notify message.
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
		// if _, err := s.db.Exec("DELETE FROM monitoring WHERE videoID = ?;", resource.ID); err != nil {
		// 	log.Println(err)
		// }
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
