package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/HTYISABUG/tgbot-youtube-notifier/src/server"
)

const channelYuuto = "UCSncTY7ruEdF36OoLv-_ZQg"

var httpPort = flag.Int("http_port", 8080, "The port for redirect server to serve from")
var httpsPort = flag.Int("https_port", 8443, "The port for main server to serve from")

func main() {
	flag.Parse()

	b, err := ioutil.ReadFile("settings.json")
	if err != nil {
		log.Fatalln(err)
	}

	var setting server.Setting
	if err := json.Unmarshal(b, &setting); err != nil {
		log.Fatalln(err)
	}

	server, err := server.NewServer(setting, *httpPort, *httpsPort)

	if err != nil {
		log.Fatalln(err)
	}

	// Handle SIGINT to cleanup program
	signalCh := make(chan os.Signal)
	signal.Notify(signalCh, os.Interrupt)
	go func() {
		<-signalCh
		log.Println("Graceful shutdown server...")
		server.Close()

		time.Sleep(time.Second * 5)

		log.Println("Goodbye")
		os.Exit(0)
	}()

	// Start webserver
	server.ListenAndServeTLS(
		setting.CertFile,
		setting.KeyFile,
	)
}
