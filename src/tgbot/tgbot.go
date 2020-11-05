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

	subCh chan<- info.SubscribeInfo
}

// NewServer returns a pointer to a new `Server` object.
func NewServer(token string, mux *http.ServeMux, subCh chan<- info.SubscribeInfo) *Server {
	server := &Server{
		client: &http.Client{},
		subCh:  subCh,

		token: token,
	}

	server.registerHandler(mux)

	return server
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

				s.subCh <- info.SubscribeInfo{
					UserID:    userID,
					ChatID:    chatID,
					ChannelID: channelID,
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
