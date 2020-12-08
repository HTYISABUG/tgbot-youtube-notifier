package server

import (
	"fmt"
	"hub"
	"log"
	"net/http"
	"strings"
	"tgbot"
	"ytapi"
)

// Server is a main server which integrated all function in this project.
type Server struct {
	hub *hub.Client
	tg  *tgbot.TgBot
	yt  *ytapi.YtAPI
	db  *database

	host      string
	httpPort  int
	httpsPort int
	serveMux  *http.ServeMux

	tgUpdateCh tgbot.UpdatesChannel
	notifyCh   <-chan hub.Feed
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
func NewServer(setting Setting, httpPort, httpsPort int) (*Server, error) {
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
	hub, notifyCh := hub.NewClient(fmt.Sprintf("%s:%d", setting.Host, httpPort), mux)

	return &Server{
		hub: hub,
		tg:  tg,
		yt:  yt,
		db:  db,

		host:      setting.Host,
		httpPort:  httpPort,
		httpsPort: httpsPort,
		serveMux:  mux,

		tgUpdateCh: tgUpdateCh,
		notifyCh:   notifyCh,
	}, nil
}

// ListenAndServeTLS starts a HTTPS server using server ServeMux
func (s *Server) ListenAndServeTLS(certFile, keyFile string) {
	// Recover all subscribed channels
	channels, err := s.db.getSubscribedChannels()
	if err != nil {
		log.Fatalln(err)
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

	// Since WebSub library can only send http link,
	// we need a redirect server to redirect request to TLS server.
	log.Println("Starting HTTP redirect server on port", s.httpPort)
	go func() {
		log.Fatalln(http.ListenAndServe(fmt.Sprintf(":%d", s.httpPort), http.HandlerFunc(s.redirectTLS)))
	}()

	// Start TLS server
	log.Println("Starting HTTPS server on port", s.httpsPort)
	log.Fatalln(http.ListenAndServeTLS(fmt.Sprintf(":%d", s.httpsPort), certFile, keyFile, s.serveMux))
}

func (s *Server) redirectTLS(w http.ResponseWriter, r *http.Request) {
	addr := fmt.Sprintf("%s:%d", s.host, s.httpsPort)
	url := "https://" + addr + r.RequestURI
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// Close stops the main server and run clean up procedures.
func (s *Server) Close() {
	channels, err := s.db.getSubscribedChannels()
	if err != nil {
		log.Fatalln(err)
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
				case "/subscribe":
					go s.subscribeHandler(update)
				case "/list":
					go s.listHandler(update)
				case "/unsubscribe":
					go s.unsubscribeHandler(update)
				}
			}
		case feed := <-s.notifyCh:
			go s.notifyHandler(feed)
		}
	}
}
