package tgbot

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

// Server is a Telegram Bot Server that can handle incoming and actively send messages.
type Server struct {
	apiClient *http.Client

	token string
}

// NewServer returns a pointer to a new `Server` object.
func NewServer(token string, mux *http.ServeMux) *Server {
	server := &Server{
		apiClient: &http.Client{},

		token: token,
	}

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
