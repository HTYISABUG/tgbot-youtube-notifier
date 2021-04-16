package server

import "database/sql"

type rowChannel struct {
	id    string
	title string
}

type rowChat struct {
	id       int64
	recorder sql.NullString
	token    sql.NullString
}

type rowNotice struct {
	videoID   string
	chatID    int64
	messageID int
}
