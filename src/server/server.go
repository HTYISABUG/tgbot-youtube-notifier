package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/HTYISABUG/tgbot-youtube-notifier/src/hub"
	"github.com/HTYISABUG/tgbot-youtube-notifier/src/tgbot"
	"github.com/HTYISABUG/tgbot-youtube-notifier/src/ytapi"
	"github.com/golang/glog"
)

// Server is a main server which integrated all function in this project.
type Server struct {
	hub *hub.Client
	tg  *tgbot.TgBot
	yt  *ytapi.YtAPI
	db  *database

	host     string
	port     int
	sslPort  int
	serveMux *http.ServeMux

	tgUpdateCh tgbot.UpdatesChannel
	notifyCh   <-chan hub.Feed

	diligentTable map[string]bool
}

// Setting represents server settings
type Setting struct {
	Host     string `json:"host"`
	BotToken string `json:"bot_token"`
	CertFile string `json:"ssl_cert"`
	KeyFile  string `json:"ssl_key"`
	DBPath   string `json:"database"`
	YtAPIKey string `json:"yt_api_key"`
}

// NewServer returns a pointer to a new `Server` object.
func NewServer(setting Setting, port, sslPort int) (*Server, error) {
	tg, err := tgbot.NewTgBot(setting.BotToken)
	if err != nil {
		return nil, err
	}

	yt := ytapi.NewYtAPI(setting.YtAPIKey)

	db, err := newDatabase(setting.DBPath)
	if err != nil {
		return nil, err
	}

	mux := new(http.ServeMux)
	tgUpdateCh := tg.ListenForWebhook("/tgbot", mux)
	hub, notifyCh := hub.NewClient(fmt.Sprintf("%s:%d", setting.Host, port), mux)

	server := &Server{
		hub: hub,
		tg:  tg,
		yt:  yt,
		db:  db,

		host:     setting.Host,
		port:     port,
		sslPort:  sslPort,
		serveMux: mux,

		tgUpdateCh: tgUpdateCh,
		notifyCh:   notifyCh,

		diligentTable: make(map[string]bool),
	}

	mux.HandleFunc("/recorder", server.recorderHandler)

	return server, nil
}

func (s *Server) initServer() {
	// Recover all subscribed channels
	channels, err := s.db.getChannels()
	if err != nil {
		glog.Fatalln(err)
	}

	for _, ch := range channels {
		s.hub.Subscribe(ch.id)
	}

	// Run hub subscription requests.
	go s.hub.Start()

	// Start service relay.
	go s.handlerRelay()

	// Initialize update scheduler.
	go s.initScheduler()
}

// ListenAndServeTLS starts a HTTPS server using server ServeMux
func (s *Server) ListenAndServe() {
	s.initServer()

	// Start server
	glog.Info("Starting server on port", s.port)
	glog.Fatalln(http.ListenAndServe(fmt.Sprintf(":%d", s.port), s.serveMux))
}

func (s *Server) ListenAndServeTLS(certFile, keyFile string) {
	s.initServer()

	go func() {
		glog.Info("Starting redirect server on port", s.port)
		glog.Fatalln(http.ListenAndServe(
			fmt.Sprintf(":%d", s.port),
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				addr := fmt.Sprintf("%s:%d", s.host, s.sslPort)
				url := "https://" + addr + r.RequestURI
				http.Redirect(w, r, url, http.StatusTemporaryRedirect)
			}),
		))
	}()

	// If host not using web server to provide ssl to tgbot service,
	// manually setup a ssl server.
	glog.Info("Starting SSL server on port", s.sslPort)
	glog.Fatalln(http.ListenAndServeTLS(fmt.Sprintf(":%d", s.sslPort), certFile, keyFile, s.serveMux))
}

// Close stops the main server and run clean up procedures.
func (s *Server) Close() {
	channels, err := s.db.getChannels()
	if err != nil {
		glog.Fatalln(err)
	}

	for _, ch := range channels {
		s.hub.Unsubscribe(ch.id)
	}
}

func (s *Server) handlerRelay() {
	for {
		select {
		case update := <-s.tgUpdateCh:
			if update.Message != nil && update.Message.Text != "" {
				elements := strings.Fields(update.Message.Text)
				switch elements[0] {
				case "/add":
					go s.chAddHandler(update)
				case "/list":
					go s.chListHandler(update)
				case "/remove":
					go s.chRemoveHandler(update)
				case "/remind":
					go s.remindHandler(update)
				case "/schedule":
					go s.scheduleHandler(update)
				case "/filter":
					go s.filterHandler(update)
				case "~autorecord":
					go s.autoRecordHandler(update)
				}
			} else if update.CallbackQuery != nil {
				go s.callbackHandler(update)
			}
		case feed := <-s.notifyCh:
			go s.noticeHandler(feed)
		}
	}
}
