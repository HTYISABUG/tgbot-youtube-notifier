package recorder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Recorder struct {
	ChatID int64
	Url    string
	Token  string
}

func (rc Recorder) Record(callbackUrl string, data map[string]interface{}) (*http.Response, error) {
	data["callback"] = callbackUrl
	data["chatID"] = rc.ChatID
	data["action"] = "record"
	return rc.request(data)
}

func (rc Recorder) Download(callbackUrl string, data map[string]interface{}) (*http.Response, error) {
	data["callback"] = callbackUrl
	data["chatID"] = rc.ChatID
	data["action"] = "download"
	return rc.request(data)
}

func (rc Recorder) request(data map[string]interface{}) (*http.Response, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", rc.Url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}

	// Add request header
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", rc.Token))

	// Setup request timeout
	client := http.Client{Timeout: 5 * time.Second}

	// Send record request to recorder
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
