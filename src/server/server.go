package server

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/HTYISABUG/tgbot-youtube-notifier/src/hub"
	"github.com/HTYISABUG/tgbot-youtube-notifier/src/recorder"
	"github.com/HTYISABUG/tgbot-youtube-notifier/src/tgbot"
	"github.com/HTYISABUG/tgbot-youtube-notifier/src/ytapi"
	"github.com/golang/glog"
)

// Server is a main server which integrated all function in this project.
type Server struct {
	Setting

	hub *hub.Client
	tg  *tgbot.TgBot
	yt  *ytapi.YtAPI
	db  *database

	serveMux *http.ServeMux

	tgUpdatesCh tgbot.UpdatesChannel
	hubFeedsCh  hub.FeedsChannel

	diligentTable map[string]bool
	recorderTable map[int64]recorder.Recorder
}

// NewServer returns a pointer to a new `Server` object.
func NewServer(setting Setting) (*Server, error) {
	// Create service multiplexer
	mux := new(http.ServeMux)

	// Initialize notifies hub
	hub, hubFeedsCh := hub.NewClient(setting.CallbackUrl(), mux)

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
	tgUpdatesCh := tg.ListenForWebhook("/tgbot", mux)

	// Initialize smain server
	server := &Server{
		Setting: setting,

		hub: hub,
		tg:  tg,
		yt:  yt,
		db:  db,

		serveMux: mux,

		tgUpdatesCh: tgUpdatesCh,
		hubFeedsCh:  hubFeedsCh,

		diligentTable: make(map[string]bool),
		recorderTable: make(map[int64]recorder.Recorder),
	}

	// Hook recoder service
	mux.HandleFunc("/recorder", server.recorderHandler)

	return server, nil
}

func (s *Server) initServer() {
	s.recoverSubscriptions()

	// Run hub subscription requests.
	go s.hub.Start()

	// Start service relay.
	go s.handlerRelay()

	// Initialize update scheduler.
	s.initScheduler()

	// Read existed recorder
	s.getRecorders()
}

func (s *Server) recoverSubscriptions() {
	channels, err := s.db.getChannels()
	if err != nil {
		glog.Fatalln(err)
	}

	for _, ch := range channels {
		s.hub.Subscribe(ch.id)
	}
}

func (s *Server) getRecorders() {
	var chats []recorder.Recorder

	err := s.db.queryResults(
		&chats,
		func(rows *sql.Rows, dest interface{}) error {
			r := dest.(*recorder.Recorder)
			return rows.Scan(&r.ChatID, &r.Url, &r.Token)
		},
		"SELECT id, recorder, token FROM chats WHERE recorder IS NOT NULL AND token IS NOT NULL;",
	)

	if err != nil {
		glog.Error(err)
		return
	}

	for _, c := range chats {
		s.recorderTable[c.ChatID] = c
	}
}

// ListenAndServeTLS starts a HTTPS server using server ServeMux
func (s *Server) ListenAndServe() {
	s.initServer()

	// Start server
	glog.Info("Starting server on port", s.ServicePort)
	glog.Fatalln(http.ListenAndServe(fmt.Sprintf(":%d", s.ServicePort), s.serveMux))
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
		case update := <-s.tgUpdatesCh:
			if update.Message != nil {
				if update.Message.ReplyToMessage != nil {
					go func() {
						err := s.filterReplyHandler(update)
						if err != nil {
							glog.Error(err)
						}
					}()
				} else if update.Message.Text != "" {
					elements := strings.Fields(update.Message.Text)
					switch elements[0] {
					case "/add":
						go s.chAddHandler(update)
					case "/list":
						go s.chListHandler(update)
					case "/remind":
						go s.remindHandler(update)
					case "/schedule":
						go s.scheduleHandler(update)
					case "/filter":
						go s.filterHandler(update)
					case "~autorc":
						go s.autoRecordHandler(update)
					case "~dl":
						go s.downloadHandler(update)
					}
				}
			} else if update.CallbackQuery != nil {
				go s.callbackHandler(update)
			}
		// Hub notifies handler
		case feed := <-s.hubFeedsCh:
			go s.noticeHandler(feed)
		}
	}
}
