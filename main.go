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

	// Start webserver
	go server.ListenAndServeTLS(
		setting.CertFile,
		setting.KeyFile,
	)

	time.Sleep(time.Second * 5)
	log.Println("Press Enter for graceful shutdown...")

	var input string
	fmt.Scanln(&input)

	time.Sleep(time.Second * 5)
}
