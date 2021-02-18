package ytapi

import (
	"context"
	"fmt"

	"github.com/golang/glog"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

// YtAPI ...
type YtAPI struct {
	*youtube.Service
}

const ytIDNumLimit = 50

// NewYtAPI ...
func NewYtAPI(apiKey string) *YtAPI {
	ctx := context.Background()
	service, err := youtube.NewService(ctx, option.WithAPIKey(apiKey), option.WithScopes(youtube.YoutubeReadonlyScope))
	if err != nil {
		glog.Fatalln("Error creating YouTube client:", err)
	}

	return &YtAPI{service}
}

// Channel is *channel* resource contains information about a YouTube
// channel.
type Channel = youtube.Channel

// GetChannel ...
func (api *YtAPI) GetChannel(channelID string, parts []string) (*Channel, error) {
	channels, err := api.GetChannels([]string{channelID}, parts)

	if err != nil {
		return nil, err
	} else if len(channels) == 0 {
		return nil, fmt.Errorf("Invalid channel ID: %s", channelID)
	} else {
		return channels[0], nil
	}
}

// GetChannels ...
func (api *YtAPI) GetChannels(channelIDs, parts []string) ([]*Channel, error) {
	var chunkLen = (len(channelIDs) + ytIDNumLimit - 1) / ytIDNumLimit
	var channels []*youtube.Channel

	for i := 0; i < chunkLen; i++ {
		begin := i * ytIDNumLimit
		end := begin + ytIDNumLimit
		if end > len(channelIDs) {
			end = len(channelIDs)
		}

		resp, err := api.getChannelListResponse(channelIDs[begin:end], parts)
		if err != nil {
			return nil, err
		}

		channels = append(channels, resp.Items...)
	}

	return channels, nil
}

func (api *YtAPI) getChannelListResponse(channelIDs, part []string) (*youtube.ChannelListResponse, error) {
	call := api.Channels.List(part)
	call = call.Id(channelIDs...)

	resp, err := call.Do()
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// Video is *video* resource represents a YouTube video.
type Video = youtube.Video

// GetVideo ...
func (api *YtAPI) GetVideo(videoID string, parts []string) (*Video, error) {
	videos, err := api.GetVideos([]string{videoID}, parts)

	if err != nil {
		return nil, err
	} else if len(videos) == 0 {
		return nil, fmt.Errorf(
			"Invalid video ID: %s. The video may not exists, not available, or be deleted",
			videoID,
		)
	} else {
		return videos[0], nil
	}
}

// GetVideos ...
func (api *YtAPI) GetVideos(videoIDs, parts []string) ([]*youtube.Video, error) {
	var chunkLen = (len(videoIDs) + ytIDNumLimit - 1) / ytIDNumLimit
	var videos []*youtube.Video

	for i := 0; i < chunkLen; i++ {
		begin := i * ytIDNumLimit
		end := begin + ytIDNumLimit
		if end > len(videoIDs) {
			end = len(videoIDs)
		}

		resp, err := api.getVideoListResponse(videoIDs[begin:end], parts)
		if err != nil {
			return nil, err
		}

		videos = append(videos, resp.Items...)
	}

	return videos, nil
}

func (api *YtAPI) getVideoListResponse(videoIDs, part []string) (*youtube.VideoListResponse, error) {
	call := api.Videos.List(part)
	call = call.Id(videoIDs...)

	resp, err := call.Do()
	if err != nil {
		return nil, err
	}

	return resp, nil
}
