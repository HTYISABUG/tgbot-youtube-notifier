package server

import "database/sql"

type Channel struct {
	id    string
	title string
}

type Notice struct {
	videoID   string
	chatID    int64
	messageID int
}

type Chat struct {
	id       int64
	recorder sql.NullString
	token    sql.NullString
}
