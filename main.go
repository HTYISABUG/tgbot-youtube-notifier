package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"server"
	"time"

	"github.com/bitly/go-simplejson"
)

const channelYuuto = "UCSncTY7ruEdF36OoLv-_ZQg"

func main() {
	b, err := ioutil.ReadFile("settings.json")
	if err != nil {
		log.Fatalln(err)
	}

	settings, err := simplejson.NewJson(b)
	if err != nil {
		log.Fatalln(err)
	}

	host := settings.Get("host").MustString()
	port := settings.Get("port").MustInt(8443)

	server := server.NewServer(host, port)

	server.Subscribe(channelYuuto)

	// Start webserver
	go server.ListenAndServe(fmt.Sprintf(":%d", port))

	time.Sleep(time.Second * 5)
	log.Println("Press Enter for graceful shutdown...")

	var input string
	fmt.Scanln(&input)

	server.Unsubscribe(channelYuuto)

	time.Sleep(time.Second * 5)
}
