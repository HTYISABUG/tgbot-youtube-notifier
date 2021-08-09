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

	host         string
	servicePort  int
	callbackPort int
	serveMux     *http.ServeMux

	tgUpdateCh tgbot.UpdatesChannel
	notifyCh   <-chan hub.Feed

	diligentTable map[string]bool
}

// Setting represents server settings
type Setting struct {
	Host         string `json:"host"`
	ServicePort  int    `json:"service_port"`
	CallbackPort int    `json:"callback_port"`

	BotToken string `json:"bot_token"`
	DBPath   string `json:"database"`
	YtAPIKey string `json:"yt_api_key"`
	CertFile string `json:"ssl_cert"`
	KeyFile  string `json:"ssl_key"`
}

// NewServer returns a pointer to a new `Server` object.
func NewServer(setting Setting) (*Server, error) {
	// Create service multiplexer
	mux := new(http.ServeMux)

	// Initialize notifies hub
	hub, notifyCh := hub.NewClient(fmt.Sprintf("%s:%d", setting.Host, setting.CallbackPort), mux)

	// Initialize tgbot api
	tg, err := tgbot.NewTgBot(setting.BotToken)
	if err != nil {
		return nil, err
	}

	// Initialize YouTube api
	yt := ytapi.NewYtAPI(setting.YtAPIKey)

	// Initialize databse
	db, err := newDatabase(setting.DBPath)
	if err != nil {
		return nil, err
	}

	// Hook tgbot service
	tgUpdateCh := tg.ListenForWebhook("/tgbot", mux)

	// Initialize smain server
	server := &Server{
		hub: hub,
		tg:  tg,
		yt:  yt,
		db:  db,

		host:         setting.Host,
		servicePort:  setting.ServicePort,
		callbackPort: setting.CallbackPort,
		serveMux:     mux,

		tgUpdateCh: tgUpdateCh,
		notifyCh:   notifyCh,

		diligentTable: make(map[string]bool),
	}

	// Hook recoder service
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
	glog.Info("Starting server on port", s.servicePort)
	glog.Fatalln(http.ListenAndServe(fmt.Sprintf(":%d", s.servicePort), s.serveMux))
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
		// Tgbot handler
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
				case "~autorec":
					go s.autoRecordHandler(update)
				case "~dl":
					go s.downloadHandler(update)
				}
			} else if update.CallbackQuery != nil {
				go s.callbackHandler(update)
			}
		// Hub notifies handler
		case feed := <-s.notifyCh:
			go s.noticeHandler(feed)
		}
	}
}
