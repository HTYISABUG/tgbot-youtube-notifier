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

var port = flag.Int("port", 8080, "The port for main server to serve from")
var settingPath = flag.String("setting", "setting.json", "The path of setting file")

var useSSL = flag.Bool("use_ssl", false, "Use standalone ssl server to serve")
var sslPort = flag.Int("ssl_port", 8443, "The port for ssl server to serve from")

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

	server, err := server.NewServer(setting, *port, *sslPort)
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

	// Start server
	if !*useSSL {
		server.ListenAndServe()
	} else {
		if setting.CertFile == "" || setting.KeyFile == "" {
			glog.Errorln("Using standalone ssl server needs to provide `ssl_cert` & `ssl_key` filepath in setting")
		}

		server.ListenAndServeTLS(setting.CertFile, setting.KeyFile)
	}
}
