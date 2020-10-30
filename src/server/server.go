package server

import (
	"hub"
	"log"
	"net/http"
)

type Server struct {
	hub *hub.Client

	serveMux *http.ServeMux
}

func NewServer(host string, port int) *Server {
	mux := new(http.ServeMux)
	return &Server{
		hub:      hub.NewClient(host, port, mux),
		serveMux: mux,
	}
}

func (server *Server) Subscribe(channelID string) {
	server.hub.Subscribe(channelID)
}

func (server *Server) ListenAndServe(addr string) {
	go server.hub.Start()
	log.Println("Starting HTTP server on", addr)
	log.Fatal(http.ListenAndServe(addr, server.serveMux))
}

func (server *Server) Unsubscribe(channelID string) {
	server.hub.Unsubscribe(channelID)
}
