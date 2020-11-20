package server

import (
	"fmt"
	"info"
	"log"
	"strconv"
	"strings"
	"tgbot"
)

func (s *Server) subscribeService(sInfo info.SubscribeInfo) {
	channelID := tgbot.Escape(sInfo.ChannelID)
	title, err := s.subscribe(sInfo)
	if title == "" {
		title = channelID
	}
	title = tgbot.Escape(title)

	var msgTemplate string
	if err == nil {
		msgTemplate = "%s %s successful."
	} else {
		log.Println(err)
		msgTemplate = "%s %s failed.\n\nIt's a internal server error,\npls contact author or resend later."
	}

	msgTemplate = tgbot.Escape(msgTemplate)

	// Send message
	if _, err := s.tg.SendMessage(sInfo.ChatID, fmt.Sprintf(
		msgTemplate,
		tgbot.ItalicText(tgbot.BordText("Subscribe")),
		tgbot.InlineLink(title, "https://www.youtube.com/channel/"+channelID),
	), map[string]interface{}{
		"disable_web_page_preview": true,
		"disable_notification":     true,
	}); err != nil {
		log.Println(err)
	}
}

func (s *Server) subscribe(sInfo info.SubscribeInfo) (string, error) {
	s.hub.Subscribe(sInfo.ChannelID)

	chInfo, err := s.api.GetChannelInfo(sInfo.ChannelID)
	if err != nil {
		return "", err
	}

	if err := s.db.Subscribe(sInfo, *chInfo); err != nil {
		return chInfo.Title, err
	}

	return chInfo.Title, nil
}

func (s *Server) listService(lInfo info.ListInfo) {
	err := s.db.GetListInfosByUserID(&lInfo)
	if err != nil {
		log.Println(err)
	} else {
		list := []string{"You already subscribed following channels:"}

		for i := 0; i < len(lInfo.ChannelIDs); i++ {
			chID := tgbot.Escape(lInfo.ChannelIDs[i])
			chTitle := tgbot.Escape(lInfo.ChannelTitles[i])
			chLink := tgbot.InlineLink(chTitle, "https://www.youtube.com/channel/"+chID)
			list = append(list, fmt.Sprintf("%2d\\|\t%s", i, chLink))
		}

		if _, err := s.tg.SendMessage(
			lInfo.ChatID,
			strings.Join(list, "\n"),
			map[string]interface{}{
				"disable_web_page_preview": true,
			}); err != nil {
			log.Println(err)
		}
	}
}

func (s *Server) unsubscribeService(uInfo info.UnsubscribeInfo) {
	lInfo := new(info.ListInfo)
	lInfo.UserID = uInfo.UserID

	err := s.db.GetListInfosByUserID(lInfo)
	if err != nil {
		log.Println(err)
	} else {
		list := []string{"You already unsubscribe following channels:"}
		set := make(map[int64]bool)

		for _, i := range uInfo.ListNumbers {
			idx, err := strconv.ParseInt(i, 10, 64)
			if err != nil || idx >= int64(len(lInfo.ChannelIDs)) {
				continue
			}
			set[idx] = true

			chID := tgbot.Escape(lInfo.ChannelIDs[idx])
			chTitle := tgbot.Escape(lInfo.ChannelTitles[idx])
			chLink := tgbot.InlineLink(chTitle, "https://www.youtube.com/channel/"+chID)
			list = append(list, chLink)
		}

		for idx := range set {
			if _, err := s.db.Exec("DELETE FROM subscribers WHERE user_id = ? AND channel_id = ?;", lInfo.UserID, lInfo.ChannelIDs[idx]); err != nil {
				log.Println(err)
				return
			}
		}

		go func() {
			rows, err := s.db.Query("SELECT channels.id FROM (channels LEFT JOIN subscribers ON channels.id = subscribers.channel_id) WHERE subscribers.user_id IS NULL;")
			if err != nil {
				log.Println(err)
				return
			}

			var cid string
			for rows.Next() {
				if err := rows.Scan(&cid); err != nil {
					log.Println(err)
					return
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

		if _, err := s.tg.SendMessage(
			lInfo.ChatID,
			strings.Join(list, "\n"),
			map[string]interface{}{
				"disable_web_page_preview": true,
			}); err != nil {
			log.Println(err)
		}
	}
}
