package server

import (
	"log"
	"time"
)

func (s *Server) initScheduler() {
	// Get initial waiting duration.
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
	if now.After(next) {
		next = next.Add(time.Hour)
	}
	dur := next.Sub(now)

	// Start scheduler after initial waiting duration.
	time.AfterFunc(dur, func() {
		go s.regularScheduler()
	})
}

func (s *Server) regularScheduler() {
	for {
		// Get all monitored video ids.
		rows, err := s.db.Query("SELECT DISTINCT videoID FROM monitoring;")
		if err != nil {
			log.Println(err)
			return
		}

		defer rows.Close()

		var videoIDs []string
		var videoID string

		for rows.Next() {
			err := rows.Scan(&videoID)
			if err != nil {
				log.Println(err)
				return
			}

			videoIDs = append(videoIDs, videoID)
		}

		if rows.Err() != nil {
			log.Println(rows.Err())
			return
		}

		// Request video resources from yt api
		resources, err := s.yt.GetVideoResources(videoIDs, []string{"snippet", "liveStreamingDetails"})
		if err != nil {
			log.Println(err)
			return
		}

		// Send or update notifies.
		for _, r := range resources {
			go s.sendVideoNotify(r)
		}

		// Wait for next hour.
		time.Sleep(time.Hour)
	}
}
