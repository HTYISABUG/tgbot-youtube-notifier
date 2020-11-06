package info

// SubscribeInfo presents who on which chat subscribes which channel
type SubscribeInfo struct {
	UserID    int
	ChatID    int64
	ChannelID string
}

type ChannelInfo struct {
	ID    string `json:"id"`
	Title string `json:"snippet>title"`
}

type NotifyInfo struct {
	VideoID   string
	ChatID    int64
	MessageID int64
	Message   string
}
