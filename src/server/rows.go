package server

type rowChannel struct {
	id    string
	title string
}

type rowUser struct {
	id     int
	chatID int64
}

type rowSubscriber struct {
	userID    int
	channelID string
}

type rowMonitoring struct {
	videoID   string
	chatID    int64
	messageID int
}
