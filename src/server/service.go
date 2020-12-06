package server

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"tgbot"
)

func (s *Server) subscribeService(update tgbot.Update) {
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

	snippet, err := s.yt.GetChannelSnippet(channel.id)
	if err != nil {
		return "", err
	}

	channel.title = snippet.Title

	if err := s.db.subscribe(user, channel); err != nil {
		return channel.title, err
	}

	return channel.title, nil
}

func (s *Server) listService(update tgbot.Update) {
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

func (s *Server) unsubscribeService(update tgbot.Update) {
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
