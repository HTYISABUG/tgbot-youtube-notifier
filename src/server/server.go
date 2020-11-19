package server

import (
	"data"
	"fmt"
	"hub"
	"info"
	"log"
	"net/http"
	"strings"
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

	tgInfoCh <-chan tgbot.TgInfo
	notifyCh <-chan hub.Entry
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
	tg, tgInfoCh := tgbot.NewServer(setting.BotToken, mux)
	hub, notifyCh := hub.NewClient(fmt.Sprintf("%s:%d", setting.Host, httpPort), mux)

	return &Server{
		hub: hub,
		tg:  tg,
		db:  db,
		api: ytapi.NewYtAPI(setting.YtAPIKey),

		host:      setting.Host,
		httpPort:  httpPort,
		httpsPort: httpsPort,
		serveMux:  mux,

		tgInfoCh: tgInfoCh,
		notifyCh: notifyCh,
	}, nil
}

// ListenAndServeTLS starts a HTTPS server using server ServeMux
func (s *Server) ListenAndServeTLS(certFile, keyFile string) {
	// Recover all subscribed channels
	channels, err := s.db.GetAllSubscibedChannelIDs()
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
	channels, err := s.db.GetAllSubscibedChannelIDs()
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
		case info := <-s.tgInfoCh:
			switch info.Type {
			case tgbot.TypeSubscribe:
				go s.subscribeService(*info.SubscribeInfo)
			case tgbot.TypeList:
				go s.listService(*info.ListInfo)
			}
		case entry := <-s.notifyCh:
			go s.notifyHandler(entry)
		}
	}
}

func (s *Server) notifyHandler(entry hub.Entry) {
	chatIDs, err := s.db.GetSubsciberChatIDsByChannelID(entry.ChannelID)
	if err != nil {
		log.Println(err)
	}

	for _, id := range chatIDs {
		if err := s.db.Notify(info.NotifyInfo{
			VideoID:   entry.VideoID,
			ChatID:    id,
			MessageID: -1,
		}, "IGNORE"); err != nil {
			log.Println(err)
		}
	}

	infos, err := s.db.GetNotifyInfosByVideoID(entry.VideoID)
	if err != nil {
		log.Println(err)
	} else {
		for _, i := range infos {
			if i.MessageID == -1 {
				message, err := s.tg.SendMessage(i.ChatID, entry2text(entry), nil)
				if err != nil {
					log.Println(err)
				} else {
					i.MessageID = message.ID
					if err := s.db.Notify(i, "REPLACE"); err != nil {
						log.Println(err)
					}
				}
			} else {
				const notModified = "Request editMessageText failed, status 400 Bad Request: message is not modified"

				if _, err := s.tg.EditMessageText(
					i.ChatID, i.MessageID, entry2text(entry), nil,
				); err != nil && !strings.HasPrefix(err.Error(), notModified) {
					log.Println(err)
				}
			}
		}
	}

	go func() {
		_, err := s.db.Exec("UPDATE channels SET title = ? WHERE id = ?;", entry.Author, entry.ChannelID)
		if err != nil {
			log.Println(err)
		}
	}()
}
