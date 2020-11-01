package tgbot

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
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
	content, _ := ioutil.ReadAll(r.Body)

	fmt.Println(string(content))

	var update update
	if err := json.Unmarshal(content, &update); err != nil {
		log.Println(err)
	}

	/* DEBUG
	fmt.Printf("%+v\n", update)
	if update.Message != nil {
		fmt.Printf("%+v\n", *update.Message)
	}
	if update.Message.From != nil {
		fmt.Printf("%+v\n", *update.Message.From)
	}
	if update.Message.From.IsBot != nil {
		fmt.Printf("%+v\n", *update.Message.From.IsBot)
	}
	if update.Message.Text != nil {
		fmt.Printf("%+v\n", *update.Message.Text)
	}
	*/
}
