package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"os"
	"os/signal"
	"time"

	"github.com/HTYISABUG/tgbot-youtube-notifier/src/server"
	"github.com/golang/glog"
)

var httpPort = flag.Int("http_port", 8080, "The port for redirect server to serve from")
var httpsPort = flag.Int("https_port", 8443, "The port for main server to serve from")
var settingPath = flag.String("setting", "setting.json", "The path of setting file")

func init() {
	flag.Parse()
	glog.CopyStandardLogTo("INFO")
}

func main() {
	b, err := ioutil.ReadFile(*settingPath)
	if err != nil {
		glog.Fatalln(err)
	}

	var setting server.Setting
	if err := json.Unmarshal(b, &setting); err != nil {
		glog.Fatalln(err)
	}

	server, err := server.NewServer(setting, *httpPort, *httpsPort)
	if err != nil {
		glog.Fatalln(err)
	}

	// Handle SIGINT to cleanup program
	signalCh := make(chan os.Signal)
	signal.Notify(signalCh, os.Interrupt)
	go func() {
		<-signalCh
		glog.Infoln("Graceful shutdown server...")
		server.Close()

		time.Sleep(time.Second * 5)

		glog.Infoln("Goodbye")
		os.Exit(0)
	}()

	// Start webserver
	server.ListenAndServeTLS(
		setting.CertFile,
		setting.KeyFile,
	)
}
