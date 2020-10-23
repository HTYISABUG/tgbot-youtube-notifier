package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/bitly/go-simplejson"
)

func main() {
	b, err := ioutil.ReadFile("settings.json")
	if err != nil {
		fmt.Println(err)
	}

	settings, err := simplejson.NewJson(b)
	if err != nil {
		fmt.Println(err)
	}

	certPath, err := settings.Get("ssl_cert").String()
	if err != nil {
		fmt.Println(err)
	}

	keyPath, err := settings.Get("ssl_key").String()
	if err != nil {
		fmt.Println(err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		content, err := ioutil.ReadAll(r.Body)
		if err != nil {
			fmt.Println(err)
		}

		fmt.Println(string(content))

		/*
			Read Telegram update

			`Update`
			This object represents an incoming update.
			At most one of the optional parameters can be present in any given update.

			Documentation: https://core.telegram.org/bots/api#update
		*/
		update, err := simplejson.NewJson(content)
		if err != nil {
			fmt.Println(err)
		}

		/*
			Get Telegram message

			`Message`
			This object represents a message.

			Documentation: https://core.telegram.org/bots/api#message
		*/
		if message, ok := update.CheckGet("message"); ok {
			if text, ok := message.CheckGet("text"); ok {
				textString := text.MustString()

				if entities, ok := message.CheckGet("entities"); ok {
					/*
						Loop through Telegram message entities

						`MessageEntity`
						This object represents one special entity in a text message. For example, hashtags, usernames, URLs, etc.
					*/
					for _, e := range entities.MustArray() {
						entity := e.(map[string]interface{})

						if entity["type"] == "url" {
							offset, _ := entity["offset"].(json.Number).Int64()
							length, _ := entity["length"].(json.Number).Int64()
							fmt.Println(textString[offset : offset+length])
							break
						}
					}
				}
			}
		}
	})

	log.Fatalln(http.ListenAndServeTLS(":8443", certPath, keyPath, nil))
}
