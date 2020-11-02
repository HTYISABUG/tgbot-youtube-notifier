package server

import (
	"data"
	"fmt"
	"hub"
	"log"
	"net/http"
	"tgbot"
)

// Server is a main server which integrated all function in this project.
type Server struct {
	hub *hub.Client
	tg  *tgbot.Server
	db  *data.Database

	host      string
	httpPort  int
	httpsPort int
	serveMux  *http.ServeMux

	notifyCh chan hub.Entry
	subCh    chan tgbot.SubscribeInfo
}

// NewServer returns a pointer to a new `Server` object.
func NewServer(host string, httpPort, httpsPort int, botToken, dbPath string) (*Server, error) {
	db, err := data.NewDatabase(dbPath)
	if err != nil {
		return nil, err
	}

	mux := new(http.ServeMux)
	notifyCh := make(chan hub.Entry, 64)
	subCh := make(chan tgbot.SubscribeInfo, 64)

	return &Server{
		hub: hub.NewClient(fmt.Sprintf("%s:%d", host, httpPort), mux, notifyCh),
		tg:  tgbot.NewServer(botToken, mux, subCh),
		db:  db,

		host:      host,
		httpPort:  httpPort,
		httpsPort: httpsPort,
		serveMux:  mux,

		notifyCh: notifyCh,
		subCh:    subCh,
	}, nil
}

// ListenAndServeTLS starts a HTTPS server using server ServeMux
func (s *Server) ListenAndServeTLS(certFile, keyFile string) {
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

func (s *Server) Subscribe(info tgbot.SubscribeInfo) error {
	s.hub.Subscribe(info.ChannelID)

	if err := s.db.Subscribe(info); err != nil {
		return err
	}

	return nil
}

func (s *Server) Unsubscribe(channelID string) {
	s.hub.Unsubscribe(channelID)
}

func (s *Server) redirectTLS(w http.ResponseWriter, r *http.Request) {
	addr := fmt.Sprintf("%s:%d", s.host, s.httpsPort)
	url := "https://" + addr + r.RequestURI
	// log.Printf("Redirect http://%v to https://%s", r.Host, addr)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

const testChatID = 283803902

func (s *Server) serviceRelay() {
	for {
		select {
		case e := <-s.notifyCh:
			// fmt.Printf("%+v\n", e) // DEBUG
			go s.tg.SendMessage(testChatID, entry2text(e), nil)
		case info := <-s.subCh:
			// fmt.Printf("%+v\n", info) // DEBUG
			if err, channelID := s.Subscribe(info), tgbot.Escape(info.ChannelID); err == nil {
				go s.tg.SendMessage(info.ChatID, fmt.Sprintf(
					"%s %s successful",
					tgbot.ItalicText(tgbot.BordText("Subscribe")),
					tgbot.InlineLink(channelID, "https://www.youtube.com/channel/"+channelID),
				), map[string]interface{}{
					"disable_web_page_preview": true,
					"disable_notification":     true,
				})
			} else {
				go s.tg.SendMessage(info.ChatID, fmt.Sprintf(
					"%s %s failed.\n\nIt's a internal server error,\npls contact author or resend later.",
					tgbot.ItalicText(tgbot.BordText("Subscribe")),
					tgbot.InlineLink(channelID, "https://www.youtube.com/channel/"+channelID),
				), map[string]interface{}{
					"disable_web_page_preview": true,
					"disable_notification":     true,
				})
			}
		}
	}
}
