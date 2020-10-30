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
}

// NewServer returns a pointer to a new `Server` object.
func NewServer(host string, httpPort, httpsPort int) *Server {
	mux := new(http.ServeMux)
	return &Server{
		hub: hub.NewClient(fmt.Sprintf("%s:%d", host, httpPort), mux),
		tg:  tgbot.NewServer(mux),

		host:      host,
		httpPort:  httpPort,
		httpsPort: httpsPort,
		serveMux:  mux,
	}
}

// ListenAndServeTLS starts a HTTPS server using server ServeMux
func (server *Server) ListenAndServeTLS(certFile, keyFile string) {
	// Run hub subscription requests
	go server.hub.Start()

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
