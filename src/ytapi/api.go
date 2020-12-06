package ytapi

import (
	"encoding/json"
	"net/http"
	"net/url"
)

// YtAPI ...
type YtAPI struct {
	*http.Client

	key string
}

const (
	ytAPIChannelURL = "https://www.googleapis.com/youtube/v3/channels"
)

// NewYtAPI ...
func NewYtAPI(key string) *YtAPI {
	return &YtAPI{&http.Client{}, key}
}

// GetChannelSnippet ...
func (api *YtAPI) GetChannelSnippet(channelID string) (ChannelSnippet, error) {
	params := make(url.Values)
	params.Set("key", api.key)
	params.Set("id", channelID)
	params.Set("part", "snippet")

	resources, err := api.makeChannelListRequest(params)
	if err != nil {
		return ChannelSnippet{}, err
	}

	return *resources[0].Snippet, nil
}

func (api *YtAPI) makeChannelListRequest(params url.Values) ([]ChannelResource, error) {
	resp, err := api.makeListRequest(ytAPIChannelURL, params)
	if err != nil {
		return nil, err
	}

	var resources []ChannelResource
	json.Unmarshal(resp.Items, &resources)

	return resources, nil
}

func (api *YtAPI) makeListRequest(rawurl string, params url.Values) (APIResponse, error) {
	url, _ := url.Parse(rawurl)
	url.RawQuery = params.Encode()

	resp, err := api.Get(url.String())
	if err != nil {
		return APIResponse{}, err
	}

	defer resp.Body.Close()

	var apiResp APIResponse
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&apiResp)

	return apiResp, err
}
