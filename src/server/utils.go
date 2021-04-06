package server

import (
	"net/http"
	"net/url"
	"strings"
)

const (
	ytHost = "www.youtube.com"
)

func followRedirectURL(rawurl string) (bool, *url.URL, error) {
	url, err := url.Parse(rawurl)
	if err != nil {
		return false, nil, nil
	}

	if url.Scheme == "" {
		url.Scheme = "https"
	}

	resp, err := http.Get(url.String())
	if err != nil {
		return false, nil, err
	} else if resp.StatusCode != http.StatusOK {
		return false, nil, nil
	} else {
		return true, resp.Request.URL, nil
	}
}

func isValidYtChannel(rawurl string) (bool, error) {
	ok, url, err := followRedirectURL(rawurl)
	if err != nil {
		return false, err
	}

	return ok && url.Host == ytHost && strings.HasPrefix(url.Path, "/channel"), nil
}

func (s *Server) isValidYtVideo(rawurl string) (bool, error) {
	ok, url, err := followRedirectURL(rawurl)
	if err != nil {
		return false, err
	}

	if ok && url.Host == ytHost && strings.HasPrefix(url.Path, "/watch") {
		videoID := url.Query()["v"][0]
		videos, err := s.yt.GetVideos([]string{videoID}, []string{"snippet"})
		if err != nil {
			return false, err
		}

		return len(videos) != 0, nil
	}

	return false, nil
}

func containsAny(s string, substrs []string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}

	return false
}
