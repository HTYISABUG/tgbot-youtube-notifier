package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"server"
	"time"
)

const channelYuuto = "UCSncTY7ruEdF36OoLv-_ZQg"

var httpPort = flag.Int("http_port", 8080, "The port for redirect server to serve from")
var httpsPort = flag.Int("https_port", 8443, "The port for main server to serve from")

type setting struct {
	Host     string `json:"host"`
	BotToken string `json:"bot_token"`
	CertFile string `json:"ssl_cert"`
	KeyFile  string `json:"ssl_key"`
}

func main() {
	flag.Parse()

	b, err := ioutil.ReadFile("settings.json")
	if err != nil {
		log.Fatalln(err)
	}

	var setting setting
	if err := json.Unmarshal(b, &setting); err != nil {
		log.Fatalln(err)
	}

	server := server.NewServer(
		setting.Host,
		*httpPort,
		*httpsPort,
		setting.BotToken,
	)

	// server.Subscribe(channelYuuto)

	// Start webserver
	go server.ListenAndServeTLS(
		setting.CertFile,
		setting.KeyFile,
	)

	time.Sleep(time.Second * 5)
	log.Println("Press Enter for graceful shutdown...")

	var input string
	fmt.Scanln(&input)

	// server.Unsubscribe(channelYuuto)

	time.Sleep(time.Second * 5)
}
