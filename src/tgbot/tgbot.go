package tgbot

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

// Server is a Telegram Bot Server that can handle incoming and actively send messages.
type Server struct {
}

// NewServer returns a pointer to a new `Server` object.
func NewServer(mux *http.ServeMux) *Server {
	server := &Server{}

	server.registerHandler(mux)

	return server
}

func (server *Server) registerHandler(mux *http.ServeMux) {
	mux.HandleFunc("/tgbot", server.handler)
}

func (server *Server) handler(w http.ResponseWriter, r *http.Request) {
	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println(string(content))
}
