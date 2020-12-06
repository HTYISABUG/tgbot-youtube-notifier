package hub

import (
	"encoding/xml"
	"fmt"
	"net/http"

	"github.com/dpup/gohubbub"
)

const googleHub = "http://pubsubhubbub.appspot.com"
const topicURLPrefix = "https://www.youtube.com/xml/feeds/videos.xml?channel_id="

// Client is a WebSub client that can receive notification from Youtube.
type Client struct {
	*gohubbub.Client

	notifyCh chan<- Entry
}

// NewClient returns a pointer to a new `Client` object.
func NewClient(addr string, mux *http.ServeMux) (*Client, <-chan Entry) {
	client := gohubbub.NewClient(addr, "Hub Client")
	notifyCh := make(chan Entry, 64)

	client.RegisterHandler(mux)

	return &Client{
		Client: client,

		notifyCh: notifyCh,
	}, notifyCh
}

// Subscribe adds a handler will be called when an update notification is received.
// If a handler already exists for the given topic it will be overridden.
func (client *Client) Subscribe(channelID string) {
	topicURL := topicURLPrefix + channelID

	if !client.HasSubscription(topicURL) {
		client.Client.Subscribe(googleHub, topicURL, client.handler)
	}
}

// Unsubscribe sends an unsubscribe notification and removes the subscription.
func (client *Client) Unsubscribe(channelID string) {
	topicURL := topicURLPrefix + channelID
	client.Client.Unsubscribe(topicURL)
}

func (client *Client) handler(contentType string, body []byte) {
	var feed feed

	err := xml.Unmarshal(body, &feed)
	if err != nil {
		fmt.Println(string(body))
	}

	client.notifyCh <- feed.Entry
}
