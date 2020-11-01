package tgbot

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/bitly/go-simplejson"
)

const tgAPIURLPrefix = "https://api.telegram.org"

// SendMessage requests to send text message to chatID
func (server *Server) SendMessage(chatID int, text string, kwargs map[string]interface{}) {
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

	server.apiRequest("sendMessage", b)
}

func (server *Server) apiRequest(method string, body []byte) {
	url := fmt.Sprintf("%s/bot%s/%s", tgAPIURLPrefix, server.token, method)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Add("Content-Type", "application/json")

	resp, err := server.httpRequester.Do(req)
	if err != nil {
		log.Printf("Request %s failed, error %v", method, err)
		return
	}

	b, _ := ioutil.ReadAll(resp.Body)
	res, _ := simplejson.NewJson(b)

	if ok, _ := res.Get("ok").Bool(); !ok {
		log.Printf(
			"Request %s failed, status %d %s",
			method,
			res.Get("error_code").MustInt(),
			res.Get("description").MustString(),
		)
	}

	// fmt.Println(string(b)) // DEBUG
}
