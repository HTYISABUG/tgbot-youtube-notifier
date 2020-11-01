package server

import (
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

	host      string
	httpPort  int
	httpsPort int
	serveMux  *http.ServeMux

	notifyCh chan hub.Entry
}

// NewServer returns a pointer to a new `Server` object.
func NewServer(host string, httpPort, httpsPort int, botToken string) *Server {
	mux := new(http.ServeMux)
	notifyCh := make(chan hub.Entry, 64)
	return &Server{
		hub: hub.NewClient(fmt.Sprintf("%s:%d", host, httpPort), mux, notifyCh),
		tg:  tgbot.NewServer(botToken, mux),

		host:      host,
		httpPort:  httpPort,
		httpsPort: httpsPort,
		serveMux:  mux,

		notifyCh: notifyCh,
	}
}

// ListenAndServeTLS starts a HTTPS server using server ServeMux
func (server *Server) ListenAndServeTLS(certFile, keyFile string) {
	// Run hub subscription requests
	go server.hub.Start()

	// Start service relay
	go server.serviceRelay()

	// Since WebSub library can only send http link,
	// we need a redirect server to redirect request to TLS server
	log.Println("Starting HTTP redirect server on port", server.httpPort)
	go func() {
		log.Fatalln(http.ListenAndServe(fmt.Sprintf(":%d", server.httpPort), http.HandlerFunc(server.redirectTLS)))
	}()

	// Start TLS server
	log.Println("Starting HTTPS server on port", server.httpsPort)
	log.Fatalln(http.ListenAndServeTLS(fmt.Sprintf(":%d", server.httpsPort), certFile, keyFile, server.serveMux))
}

func (server *Server) Subscribe(channelID string) {
	server.hub.Subscribe(channelID)
}

func (server *Server) Unsubscribe(channelID string) {
	server.hub.Unsubscribe(channelID)
}

func (server *Server) redirectTLS(w http.ResponseWriter, r *http.Request) {
	addr := fmt.Sprintf("%s:%d", server.host, server.httpsPort)
	url := "https://" + addr + r.RequestURI
	// log.Printf("Redirect http://%v to https://%s", r.Host, addr)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

const testChatID = 283803902

func (server *Server) serviceRelay() {
	for e := range server.notifyCh {
		// fmt.Printf("%+v\n", e) // DEBUG
		server.tg.SendMessage(testChatID, entry2text(e), nil)
	}
}
