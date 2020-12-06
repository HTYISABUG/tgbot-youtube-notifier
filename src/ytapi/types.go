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
