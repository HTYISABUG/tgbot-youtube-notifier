package ytapi

import "encoding/json"

// APIResponse is a response from the YouTube API with the result
// stored raw.
type APIResponse struct {
	Kind          string          `json:"kind"`
	Etag          string          `json:"etag"`
	NextPageToken string          `json:"nextPageToken"`
	PrevPageToken string          `json:"prevPageToken"`
	PageInfo      *PageInfo       `json:"pageInfo"`
	Items         json.RawMessage `json:"items"`
}

// PageInfo object encapsulates paging information for the result
// set.
type PageInfo struct {
	TotalResults   int `json:"totalResults"`
	ResultsPerPage int `json:"resultsPerPage"`
}

// ChannelResource ...
type ChannelResource struct {
	Kind                string          `json:"kind"`
	Etag                string          `json:"etag"`
	ID                  string          `json:"id"`
	Snippet             *ChannelSnippet `json:"snippet"`
	ContentDetails      json.RawMessage `json:"contentDetails"`
	Statistics          json.RawMessage `json:"statistics"`
	TopicDetails        json.RawMessage `json:"topicDetails"`
	Status              json.RawMessage `json:"status"`
	BrandingSettings    json.RawMessage `json:"brandingSettings"`
	AuditDetails        json.RawMessage `json:"auditDetails"`
	ContentOwnerDetails json.RawMessage `json:"contentOwnerDetails"`
	Localizations       json.RawMessage `json:"localizations"`
}

// ChannelSnippet object contains basic details about the channel,
// such as its title, description, and thumbnail images.
type ChannelSnippet struct {
	Title           string          `json:"title"`
	Description     string          `json:"description"`
	CustomURL       string          `json:"customUrl"`
	PublishedAt     string          `json:"publishedAt"`
	Thumbnails      json.RawMessage `json:"thumbnails"`
	DefaultLanguage string          `json:"defaultLanguage"`
	Localized       json.RawMessage `json:"localized"`
	Country         string          `json:"country"`
}

// VideoResource ...
type VideoResource struct {
	Kind                 string                     `json:"kind"`
	Etag                 string                     `json:"etag"`
	ID                   string                     `json:"id"`
	Snippet              *VideoSnippet              `json:"snippet"`
	ContentDetails       json.RawMessage            `json:"contentDetails"`
	Status               json.RawMessage            `json:"status"`
	Statistics           json.RawMessage            `json:"statistics"`
	Player               json.RawMessage            `json:"player"`
	TopicDetails         json.RawMessage            `json:"topicDetails"`
	RecordingDetails     json.RawMessage            `json:"recordingDetails"`
	FileDetails          json.RawMessage            `json:"fileDetails"`
	ProcessingDetails    json.RawMessage            `json:"processingDetails"`
	Suggestions          json.RawMessage            `json:"suggestions"`
	LiveStreamingDetails *VideoLiveStreamingDetails `json:"liveStreamingDetails"`
	Localizations        json.RawMessage            `json:"localizations"`
}

// IsLiveBroadcast ...
func (r *VideoResource) IsLiveBroadcast() bool {
	return r.LiveStreamingDetails != nil
}

// IsUpcomingLiveBroadcast ...
func (r *VideoResource) IsUpcomingLiveBroadcast() bool {
	return r.IsLiveBroadcast() &&
		r.LiveStreamingDetails.ActualEndTime == "" &&
		r.LiveStreamingDetails.ActualStartTime == "" &&
		r.LiveStreamingDetails.ScheduledStartTime != ""
}

// IsLiveLiveBroadcast ...
func (r *VideoResource) IsLiveLiveBroadcast() bool {
	return r.IsLiveBroadcast() &&
		r.LiveStreamingDetails.ActualEndTime == "" &&
		r.LiveStreamingDetails.ActualStartTime != ""
}

// IsCompletedLiveBroadcast ...
func (r *VideoResource) IsCompletedLiveBroadcast() bool {
	return r.IsLiveBroadcast() &&
		r.LiveStreamingDetails.ActualEndTime != ""
}

// VideoSnippet object contains basic details about the video,
// such as its title, description, and category.
type VideoSnippet struct {
	PublishedAt          string          `json:"publishedAt"`
	Title                string          `json:"title"`
	ChannelID            string          `json:"channelId"`
	Description          string          `json:"description"`
	Thumbnails           json.RawMessage `json:"thumbnails"`
	ChannelTitle         string          `json:"channelTitle"`
	Tags                 json.RawMessage `json:"tags"`
	CategoryID           string          `json:"categoryId"`
	LiveBroadcastContent string          `json:"liveBroadcastContent"`
	DefaultLanguage      string          `json:"defaultLanguage"`
	Localized            json.RawMessage `json:"localized"`
	DefaultAudioLanguage string          `json:"defaultAudioLanguage"`
}

// VideoLiveStreamingDetails object contains metadata about a live video broadcast.
// The object will only be present in a video resource if the video is an upcoming,
// live, or completed live broadcast.
type VideoLiveStreamingDetails struct {
	ActualStartTime    string `json:"actualStartTime"`
	ActualEndTime      string `json:"actualEndTime"`
	ScheduledStartTime string `json:"scheduledStartTime"`
	ScheduledEndTime   string `json:"scheduledEndTime"`
	ConcurrentViewers  int64  `json:"concurrentViewers"`
	ActiveLiveChatID   string `json:"activeLiveChatId"`
}
