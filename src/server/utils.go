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

	if ok && url.Host == ytHost && strings.HasPrefix(url.Path, "/channel") {
		return true, nil
	}

	return false, nil
}

func (s *Server) isValidYtVideo(rawurl string) (bool, error) {
	ok, url, err := followRedirectURL(rawurl)
	if err != nil {
		return false, err
	}

	if ok && url.Host == ytHost && strings.HasPrefix(url.Path, "/watch") {
		videoID := url.Query()["v"][0]
		resources, err := s.yt.GetVideoResources([]string{videoID}, []string{"snippet"})
		if err != nil {
			return false, err
		} else if len(resources) != 0 {
			return true, nil
		} else {
			return false, nil
		}
	}

	return false, nil
}
