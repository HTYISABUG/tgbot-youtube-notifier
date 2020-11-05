package server

import (
	"data"
	"fmt"
	"hub"
	"log"
	"net/http"
	"tgbot"
	"ytapi"
)

// Server is a main server which integrated all function in this project.
type Server struct {
	hub *hub.Client
	tg  *tgbot.Server
	db  *data.Database
	api *ytapi.YtAPI

	host      string
	httpPort  int
	httpsPort int
	serveMux  *http.ServeMux

	notifyCh chan hub.Entry
	subCh    chan tgbot.SubscribeInfo
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
	db, err := data.NewDatabase(setting.DBPath)
	if err != nil {
		return nil, err
	}

	mux := new(http.ServeMux)
	notifyCh := make(chan hub.Entry, 64)
	subCh := make(chan tgbot.SubscribeInfo, 64)

	return &Server{
		hub: hub.NewClient(fmt.Sprintf("%s:%d", setting.Host, httpPort), mux, notifyCh),
		tg:  tgbot.NewServer(setting.BotToken, mux, subCh),
		db:  db,
		api: ytapi.NewYtAPI(setting.YtAPIKey),

		host:      setting.Host,
		httpPort:  httpPort,
		httpsPort: httpsPort,
		serveMux:  mux,

		notifyCh: notifyCh,
		subCh:    subCh,
	}, nil
}

// ListenAndServeTLS starts a HTTPS server using server ServeMux
func (s *Server) ListenAndServeTLS(certFile, keyFile string) {
	// Recover all subscribed channels
	channels, err := s.db.GetSubscibedChannels()
	if err != nil {
		log.Fatalln(err)
	}

	for _, cid := range channels {
		s.hub.Subscribe(cid)
	}

	// Run hub subscription requests
	go s.hub.Start()

	// Start service relay
	go s.serviceRelay()

	// Since WebSub library can only send http link,
	// we need a redirect server to redirect request to TLS server
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

// Close stops the main server and run clean up procedures
func (s *Server) Close() {
	channels, err := s.db.GetSubscibedChannels()
	if err != nil {
		log.Fatalln(err)
	}

	for _, cid := range channels {
		s.hub.Unsubscribe(cid)
	}
}

func (s *Server) serviceRelay() {
	for {
		select {
		case info := <-s.subCh:
			go s.subscribeService(info)
		case entry := <-s.notifyCh:
			chatIDs, err := s.db.GetSubsciberChats(entry.ChannelID)
			if err != nil {
				log.Println(err)
			}

			for _, id := range chatIDs {
				go func(chatID int64, entry hub.Entry) {
					if _, err := s.tg.SendMessage(chatID, entry2text(entry), nil); err != nil {
						log.Println(err)
					}
				}(id, entry)
			}
		}
	}
}

func (s *Server) subscribeService(info tgbot.SubscribeInfo) {
	channelID := tgbot.Escape(info.ChannelID)
	title, err := s.subscribe(info)
	if title == "" {
		title = channelID
	}
	title = tgbot.Escape(title)

	var msgTemplate string
	if err == nil {
		msgTemplate = "%s %s successful."
	} else {
		log.Println(err)
		msgTemplate = "%s %s failed.\n\nIt's a internal server error,\npls contact author or resend later."
	}

	msgTemplate = tgbot.Escape(msgTemplate)

	// Send message
	if _, err := s.tg.SendMessage(info.ChatID, fmt.Sprintf(
		msgTemplate,
		tgbot.ItalicText(tgbot.BordText("Subscribe")),
		tgbot.InlineLink(title, "https://www.youtube.com/channel/"+channelID),
	), map[string]interface{}{
		"disable_web_page_preview": true,
		"disable_notification":     true,
	}); err != nil {
		log.Println(err)
	}
}

func (s *Server) subscribe(subInfo tgbot.SubscribeInfo) (string, error) {
	s.hub.Subscribe(subInfo.ChannelID)

	chInfo, err := s.api.GetChannelInfo(subInfo.ChannelID)
	if err != nil {
		return "", err
	}

	if err := s.db.Subscribe(subInfo, *chInfo); err != nil {
		return chInfo.Title, err
	}

	return chInfo.Title, nil
}
