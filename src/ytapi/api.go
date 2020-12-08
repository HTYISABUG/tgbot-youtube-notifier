package ytapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
)

// YtAPI ...
type YtAPI struct {
	*http.Client

	key string
}

const (
	apiChannelsURL = "https://www.googleapis.com/youtube/v3/channels"
	apiVideosURL   = "https://www.googleapis.com/youtube/v3/videos"
)

// NewYtAPI ...
func NewYtAPI(key string) *YtAPI {
	return &YtAPI{&http.Client{}, key}
}

// GetChannelResource ...
func (api *YtAPI) GetChannelResource(channelID string) (ChannelResource, error) {
	resources, err := api.getChannelResources(
		[]string{channelID},
		[]string{"snippet"},
	)

	if err != nil {
		return ChannelResource{}, err
	} else if len(resources) == 0 {
		return ChannelResource{}, errors.New("Invalid channel ID")
	} else {
		return resources[0], nil
	}
}

func (api *YtAPI) getChannelResources(channelIDs, parts []string) ([]ChannelResource, error) {
	params := make(url.Values)
	params.Set("key", api.key)
	params.Set("id", strings.Join(channelIDs, ","))
	params.Set("part", strings.Join(parts, ","))

	resources, err := api.makeChannelListRequest(params)
	if err != nil {
		return []ChannelResource{}, err
	}

	return resources, nil
}

func (api *YtAPI) makeChannelListRequest(params url.Values) ([]ChannelResource, error) {
	resp, err := api.makeListRequest(apiChannelsURL, params)
	if err != nil {
		return nil, err
	}

	var resources []ChannelResource
	json.Unmarshal(resp.Items, &resources)

	return resources, nil
}

// GetVideoResource ...
func (api *YtAPI) GetVideoResource(videoID string) (VideoResource, error) {
	resources, err := api.GetVideoResources(
		[]string{videoID},
		[]string{"snippet", "liveStreamingDetails"},
	)

	if err != nil {
		return VideoResource{}, err
	} else if len(resources) == 0 {
		return VideoResource{}, errors.New("Invalid video ID: " + videoID)
	} else {
		return resources[0], nil
	}
}

// GetVideoResources ...
func (api *YtAPI) GetVideoResources(videoIDs, parts []string) ([]VideoResource, error) {
	params := make(url.Values)
	params.Set("key", api.key)
	params.Set("id", strings.Join(videoIDs, ","))
	params.Set("part", strings.Join(parts, ","))

	resources, err := api.makeVideoListRequest(params)
	if err != nil {
		return []VideoResource{}, err
	}

	return resources, nil
}

func (api *YtAPI) makeVideoListRequest(params url.Values) ([]VideoResource, error) {
	resp, err := api.makeListRequest(apiVideosURL, params)
	if err != nil {
		return nil, err
	}

	var resources []VideoResource
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
