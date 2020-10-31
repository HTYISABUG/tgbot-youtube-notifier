package hub

// Feed is for containing WebSub notification.
type Feed struct {
	Entry Entry `xml:"entry"`
}

// Entry is for containing entry element in Feed.
type Entry struct {
	VideoID   string `xml:"http://www.youtube.com/xml/schemas/2015 videoId"`
	ChannelID string `xml:"http://www.youtube.com/xml/schemas/2015 channelId"`
	Title     string `xml:"title"`
	Author    string `xml:"author>name"`
	Published string `xml:"published"`
	Updated   string `xml:"updated"`
}
