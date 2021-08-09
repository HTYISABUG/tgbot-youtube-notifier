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

var settingPath = flag.String("setting", "setting.json", "The path of setting file")

func main() {
	flag.Parse()                   // Parse cmd arguments.
	glog.CopyStandardLogTo("INFO") // Redirect std log to glog

	// Load setting file
	b, err := ioutil.ReadFile(*settingPath)
	if err != nil {
		glog.Fatalln(err)
	}

	var setting server.Setting
	if err := json.Unmarshal(b, &setting); err != nil {
		glog.Fatalln(err)
	}

	// Initialize server
	server, err := server.NewServer(setting)
	if err != nil {
		glog.Fatalln(err)
	}

	// Handle SIGINT to cleanup program
	signalCh := make(chan os.Signal)
	signal.Notify(signalCh, os.Interrupt)
	go func() {
		<-signalCh
		glog.Info("Graceful shutdown server...")
		server.Close()

		time.Sleep(time.Second * 5)

		glog.Info("Goodbye")
		os.Exit(0)
	}()

	// Start server
	server.ListenAndServe()
}
