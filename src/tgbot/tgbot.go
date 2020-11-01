package tgbot

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type httpRequester interface {
	Do(req *http.Request) (resp *http.Response, err error)
}

// Server is a Telegram Bot Server that can handle incoming and actively send messages.
type Server struct {
	httpRequester httpRequester

	token string
}

// NewServer returns a pointer to a new `Server` object.
func NewServer(token string, mux *http.ServeMux) *Server {
	server := &Server{
		httpRequester: &http.Client{},

		token: token,
	}

	server.registerHandler(mux)

	return server
}

func (server *Server) registerHandler(mux *http.ServeMux) {
	mux.HandleFunc("/tgbot", server.handler)
}

func (server *Server) handler(w http.ResponseWriter, r *http.Request) {
	content, _ := ioutil.ReadAll(r.Body)

	fmt.Println(string(content))

	var update update
	if err := json.Unmarshal(content, &update); err != nil {
		log.Println(err)
		return
	}

	if update.Message != nil && update.Message.Text != nil {
		elements := strings.Fields(*update.Message.Text)
		switch elements[0] {
		case "/subscribe":
			server.subscribeHandler(&update)
		}
	}
}

func (server *Server) subscribeHandler(update *update) {
	if imNotARobot(update.Message.From) && isPrivate(update.Message.Chat) {
		elements := strings.Fields(*update.Message.Text)
		var results []string
		for _, e := range elements[1:] {
			if b, err := server.isValidYtChannel(e); err == nil && b {
				url, _ := url.Parse(e)

				channelID := strings.Split(url.Path, "/")[2]
				channelID = regexp.QuoteMeta(channelID)
				e = regexp.QuoteMeta(e)

				results = append(results, fmt.Sprintf(
					"%s %s \\.\\.\\.",
					ItalicText(BordText("Subscribing")),
					InlineLink(channelID, e),
				))
			} else if err != nil {
				log.Println(err)

				e := regexp.QuoteMeta(e)
				results = append(results, fmt.Sprintf("Subscribe %s failed, internal server error", e))
			} else if !b {
				e := regexp.QuoteMeta(e)
				results = append(results, fmt.Sprintf("%s is not a valid YouTube channel", e))
			}
		}

		server.SendMessage(
			update.Message.Chat.ID,
			strings.Join(results, "\n"),
			map[string]interface{}{"disable_web_page_preview": true},
		)

		// fmt.Println(strings.Join(results, "\n"))
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

func (server *Server) isValidYtChannel(rawurl string) (bool, error) {
	url, err := url.Parse(rawurl)
	if err != nil {
		return false, err
	}

	if url.Scheme == "" {
		url.Scheme = "https"
		url, _ = url.Parse(url.String())
	}

	if url.Scheme == "http" || url.Scheme == "https" &&
		(url.Host == ytHost || url.Host == ytHostFull) &&
		strings.HasPrefix(url.Path, "/channel") {
		req, _ := http.NewRequest("GET", url.String(), nil)
		resp, err := server.httpRequester.Do(req)
		if err != nil {
			return false, err
		} else if resp.StatusCode == http.StatusOK {
			return true, nil
		}
	}

	return false, nil
}
