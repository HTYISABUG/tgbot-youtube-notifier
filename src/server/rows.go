package server

type rowChannel struct {
	id    string
	title string
}

type rowChat struct {
	id    int64
	admin bool
}

type rowSubscriber struct {
	chatID    int64
	channelID string
}

type rowMonitoring struct {
	videoID   string
	chatID    int64
	messageID int
}
