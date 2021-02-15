package server

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/HTYISABUG/tgbot-youtube-notifier/src/tgbot"
	"github.com/HTYISABUG/tgbot-youtube-notifier/src/ytapi"
)

func (s *Server) initScheduler() {
	// Update all notifies first.
	s.updateNotifies()

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

const updateFrequency = time.Hour

func (s *Server) regularScheduler() {
	for {
		s.updateNotifies()

		// Wait for next period.
		time.Sleep(updateFrequency)
	}
}

func (s *Server) updateNotifies() {
	// Get all monitored video ids.
	var videoIDs []string

	err := s.db.queryResults(
		&videoIDs,
		func(rows *sql.Rows, dest interface{}) error {
			r := dest.(*string)
			return rows.Scan(r)
		},
		"SELECT DISTINCT videoID FROM monitoring;",
	)

	if err != nil {
		log.Println(err)
		return
	}

	// Request video resources from yt api
	videos, err := s.yt.GetVideos(videoIDs, []string{"snippet", "liveStreamingDetails"})
	if err != nil {
		log.Println(err)
		return
	}

	for _, v := range videos {
		// Send or update notifies.
		go s.sendVideoNotify(v)

		s.tryDiligentScheduler(v)
	}
}

func (s *Server) tryDiligentScheduler(video *ytapi.Video) {
	if s.isDiligentCondition(video) {
		s.diligentTable[video.Id] = true

		t, _ := time.Parse(time.RFC3339, video.LiveStreamingDetails.ScheduledStartTime)
		remains := time.Until(t)

		videoID := video.Id

		// Run diligent scheduler
		time.AfterFunc(getWaitingDuration(remains), func() {
			go func() {
				s.diligentScheduler(videoID)
				delete(s.diligentTable, videoID)
			}()
		})
	}
}

func (s *Server) isDiligentCondition(v *ytapi.Video) bool {
	if ytapi.IsUpcomingLiveBroadcast(v) {
		t, _ := time.Parse(time.RFC3339, v.LiveStreamingDetails.ScheduledStartTime)
		remains := time.Until(t)

		// Check is remaining time longer than update frequency & not in diligent table
		if _, ok := s.diligentTable[v.Id]; remains <= updateFrequency && !ok {
			return true
		}
	}

	return false
}

func (s *Server) diligentScheduler(videoID string) {
	log.Printf("DEBUG: running " + ytVideoURLPrefix + videoID + " diligent scheduler")

	for {
		time.Sleep(time.Second)

		// Get video resource & update notifies.
		v, err := s.yt.GetVideo(videoID, []string{"snippet", "liveStreamingDetails"})
		if err != nil {
			log.Println(err)
			return
		}

		s.sendVideoNotify(v)

		// Get remaining time
		t, _ := time.Parse(time.RFC3339, v.LiveStreamingDetails.ScheduledStartTime)
		remains := time.Until(t)

		if remains > updateFrequency {
			// If still have enough time, stop diligent scheduler.
			return
		} else if ytapi.IsLiveLiveBroadcast(v) {
			// If live already start, stop diligent scheduler & send notifies.
			mMessages, err := s.db.getMonitoringByVideoID(v.Id)
			if err != nil {
				log.Println(err)
				return
			}

			for _, mMsg := range mMessages {
				msgConfig := tgbot.NewMessage(mMsg.chatID, fmt.Sprintf(
					"%s\n%s",
					tgbot.EscapeText(v.Snippet.ChannelTitle+" is now live!"),
					tgbot.InlineLink(
						tgbot.BordText(tgbot.EscapeText(v.Snippet.Title)),
						ytVideoURLPrefix+tgbot.EscapeText(v.Id),
					),
				))
				msgConfig.DisableWebPagePreview = true

				_, err := s.tg.Send(msgConfig)
				if err != nil {
					log.Println(err)
					fmt.Println(msgConfig.Text)
				}
			}

			return
		}

		time.Sleep(getWaitingDuration(remains))

		// WTF, scheduled start time has arrived but live still not started!
		if remains <= 0 {
			log.Printf("DEBUG: running " + ytVideoURLPrefix + v.Id + " tolerance section")
			log.Printf("DEBUG: already " + (-remains).String() + " has elapsed")

			// Well, lets wait for 1 more minutes.
			time.Sleep(1 * time.Minute)
		}
	}
}

func getWaitingDuration(t time.Duration) time.Duration {
	var interval = [...]time.Duration{30 * time.Minute, 15 * time.Minute, 5 * time.Minute, 0}

	for _, v := range interval {
		if t > v {
			return t - v
		}
	}

	return 0
}
