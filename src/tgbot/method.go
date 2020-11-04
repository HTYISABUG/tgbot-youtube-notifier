package tgbot

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/bitly/go-simplejson"
)

const tgAPIURLPrefix = "https://api.telegram.org"

// SendMessage requests to send text message to chatID
func (s *Server) SendMessage(chatID int64, text string, kwargs map[string]interface{}) error {
	body := simplejson.New()
	body.Set("chat_id", chatID)
	body.Set("text", text)
	body.Set("parse_mode", "MarkdownV2")

	if kwargs != nil {
		for k, v := range kwargs {
			body.Set(k, v)
		}
	}

	b, _ := body.MarshalJSON()

	return s.apiRequest("sendMessage", b)
}

func (s *Server) apiRequest(method string, body []byte) error {
	url := fmt.Sprintf("%s/bot%s/%s", tgAPIURLPrefix, s.token, method)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Add("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("Request %s failed, error %v", method, err)
	}

	b, _ := ioutil.ReadAll(resp.Body)
	res, _ := simplejson.NewJson(b)

	if ok, _ := res.Get("ok").Bool(); !ok {
		return fmt.Errorf(
			"Request %s failed, status %d %s",
			method,
			res.Get("error_code").MustInt(),
			res.Get("description").MustString(),
		)
	}

	return nil
}
