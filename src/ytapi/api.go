package ytapi

import (
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/bitly/go-simplejson"
)

type YtAPI struct {
	*http.Client

	key string
}

const (
	ytAPIChannelURL = "https://www.googleapis.com/youtube/v3/channels"
)

func NewYtAPI(key string) *YtAPI {
	return &YtAPI{&http.Client{}, key}
}

func (api *YtAPI) GetChannelInfo(channelID string) (*ChannelInfo, error) {
	url, _ := url.Parse(ytAPIChannelURL)
	q := url.Query()
	q.Set("key", api.key)
	q.Set("id", channelID)
	q.Set("part", "snippet")
	url.RawQuery = q.Encode()

	resp, err := api.Get(url.String())
	if err != nil {
		return nil, err
	}

	b, _ := ioutil.ReadAll(resp.Body)
	body, _ := simplejson.NewJson(b)

	var info ChannelInfo
	info.ID = body.Get("items").GetIndex(0).Get("id").MustString()
	info.Title = body.Get("items").GetIndex(0).Get("snippet").Get("title").MustString()

	return &info, nil
}
