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
	httpPort := settings.Get("http_port").MustInt(8080)
	httpsPort := settings.Get("https_port").MustInt(8443)

	certFile, err := settings.Get("ssl_cert").String()
	if err != nil {
		log.Fatalln(err)
	}

	keyFile, err := settings.Get("ssl_key").String()
	if err != nil {
		log.Fatalln(err)
	}

	server := server.NewServer(host, httpPort, httpsPort)

	server.Subscribe(channelYuuto)

	// Start webserver
	go server.ListenAndServeTLS(certFile, keyFile)

	time.Sleep(time.Second * 5)
	log.Println("Press Enter for graceful shutdown...")

	var input string
	fmt.Scanln(&input)

	server.Unsubscribe(channelYuuto)

	time.Sleep(time.Second * 5)
}
