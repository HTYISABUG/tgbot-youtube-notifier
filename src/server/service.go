package server

import (
	"fmt"
	"info"
	"log"
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
