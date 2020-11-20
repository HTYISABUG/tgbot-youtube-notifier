package tgbot

import (
	"encoding/json"
	"fmt"
	"info"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
)

// Server is a Telegram Bot Server that can handle incoming and actively send messages.
type Server struct {
	client *http.Client

	token string

	infoCh chan<- TgInfo
}

// TgInfo transmits info type and data to main server
type TgInfo struct {
	Type int

	SubscribeInfo   *info.SubscribeInfo
	ListInfo        *info.ListInfo
	UnsubscribeInfo *info.UnsubscribeInfo
}

const (
	TypeSubscribe = iota
	TypeList
	TypeUnsubscribe
)

// NewServer returns a pointer to a new `Server` object.
func NewServer(token string, mux *http.ServeMux) (*Server, <-chan TgInfo) {
	infoCh := make(chan TgInfo, 64)
	server := &Server{
		client: &http.Client{},
		infoCh: infoCh,

		token: token,
	}

	server.registerHandler(mux)

	return server, infoCh
}

func (s *Server) registerHandler(mux *http.ServeMux) {
	mux.HandleFunc("/tgbot", s.handler)
}

func (s *Server) handler(w http.ResponseWriter, r *http.Request) {
	var update update
	content, _ := ioutil.ReadAll(r.Body)
	json.Unmarshal(content, &update)

	if update.Message != nil && update.Message.Text != nil {
		elements := strings.Fields(*update.Message.Text)
		switch elements[0] {
		case "/subscribe":
			s.subscribeHandler(&update)
		case "/list":
			s.listHandler(&update)
		case "/unsubscribe":
			s.unsubscribeHandler(&update)
		}
	}
}

func (s *Server) subscribeHandler(update *update) {
	if imNotARobot(update.Message.From) && isPrivate(update.Message.Chat) {
		elements := strings.Fields(*update.Message.Text)

		var results []string
		for _, e := range elements[1:] {
			if b, err := s.isValidYtChannel(e); err == nil && b {
				url, _ := url.Parse(e)

				userID := update.Message.From.ID
				chatID := update.Message.Chat.ID
				channelID := strings.Split(url.Path, "/")[2]

				s.infoCh <- TgInfo{
					Type: TypeSubscribe,
					SubscribeInfo: &info.SubscribeInfo{
						UserID:    userID,
						ChatID:    chatID,
						ChannelID: channelID,
					},
				}

				channelID, e = Escape(channelID), Escape(e)
				results = append(results, fmt.Sprintf(
					"%s %s",
					ItalicText(BordText("Subscribing")),
					InlineLink(channelID, e),
				))

			} else if err != nil {
				log.Println(err)

				e := Escape(e)
				results = append(results, fmt.Sprintf("Subscribe %s failed, internal server error", e))
			} else if !b {
				e := Escape(e)
				results = append(results, fmt.Sprintf("%s is not a valid YouTube channel", e))
			}
		}

		if _, err := s.SendMessage(
			update.Message.Chat.ID,
			strings.Join(results, "\n"),
			map[string]interface{}{
				"disable_web_page_preview": true,
				"disable_notification":     true,
			},
		); err != nil {
			log.Println(err)
		}
	}
}

func imNotARobot(user *user) bool {
	return user != nil && user.IsBot != nil && !*user.IsBot
}

func isPrivate(chat *chat) bool {
	return chat != nil && chat.Type == "private"
}

const (
	ytHost     = "youtube.com"
	ytHostFull = "www.youtube.com"
)

func (s *Server) isValidYtChannel(rawurl string) (bool, error) {
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
		resp, err := s.client.Get(url.String())

		if err != nil {
			return false, err
		} else if resp.StatusCode == http.StatusOK {
			return true, nil
		}
	}

	return false, nil
}

func (s *Server) listHandler(update *update) {
	userID := update.Message.From.ID
	s.infoCh <- TgInfo{
		Type:     TypeList,
		ListInfo: &info.ListInfo{UserID: userID},
	}
}

func (s *Server) unsubscribeHandler(update *update) {
	elements := strings.Fields(*update.Message.Text)

	if len(elements) == 1 {
		if _, err := s.SendMessage(
			update.Message.Chat.ID,
			"Please use /list to find the channel numbers "+
				"which you want to unsubscribe\\. "+
				"Then use `\\/unsubscribe <number\\> \\.\\.\\.` to unsubscribe\\.",
			nil); err != nil {
			log.Println(err)
		}
	} else {
		s.infoCh <- TgInfo{
			Type: TypeUnsubscribe,
			UnsubscribeInfo: &info.UnsubscribeInfo{
				UserID:      update.Message.From.ID,
				ListNumbers: elements[1:],
			},
		}
	}
}
