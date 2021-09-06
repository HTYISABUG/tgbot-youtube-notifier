package server

import "fmt"

type Setting struct {
	Host         string `json:"host"`
	ServicePort  int    `json:"service_port"`
	CallbackPort int    `json:"callback_port"`

	BotToken string `json:"bot_token"`
	DBPath   string `json:"database"`
	YtAPIKey string `json:"yt_api_key"`
	CertFile string `json:"ssl_cert"`
	KeyFile  string `json:"ssl_key"`
}

func (s Setting) CallbackUrl() string {
	return fmt.Sprintf("%s:%d", s.Host, s.CallbackPort)
}
