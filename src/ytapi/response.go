package ytapi

type ChannelInfo struct {
	ID    string `json:"id"`
	Title string `json:"snippet>title"`
}
