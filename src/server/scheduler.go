package server

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/HTYISABUG/tgbot-youtube-notifier/src/tgbot"
	"github.com/HTYISABUG/tgbot-youtube-notifier/src/ytapi"
	"github.com/golang/glog"
	"google.golang.org/api/youtube/v3"
)

const updateFrequency = time.Hour

func (s *Server) initScheduler() {
	// Update all notifies first.
	s.updateNotifies()

	// Get initial waiting duration.
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
	for now.After(next) {
		next = next.Add(updateFrequency)
	}
	dur := next.Sub(now)

	// Start scheduler after initial waiting duration.
	time.AfterFunc(dur, func() {
		go s.regularScheduler()
	})
}

func (s *Server) regularScheduler() {
	for {
		s.updateNotifies()

		// TODO: Update recorder status

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
		"SELECT DISTINCT videoID FROM notices;",
	)

	if err != nil {
		glog.Error(err)
		return
	}

	// Request video resources from yt api
	videos, err := s.yt.GetVideos(videoIDs, []string{"snippet", "liveStreamingDetails"})
	if err != nil {
		glog.Warning(err)
		return
	}

	for _, v := range videos {
		// Send or update notifies.
		go func(v *youtube.Video) {
			s.sendNotices(v)
			s.tryDiligentScheduler(v)
		}(v)
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
	glog.Info("Running " + ytVideoURLPrefix + videoID + " diligent scheduler")

	for {
		time.Sleep(time.Second)

		// Get video resource & update notifies.
		v, err := s.yt.GetVideo(videoID, []string{"snippet", "liveStreamingDetails"})
		if err != nil {
			glog.Warning(err)
			return
		}

		s.sendNotices(v)

		// Get remaining time
		t, _ := time.Parse(time.RFC3339, v.LiveStreamingDetails.ScheduledStartTime)
		remains := time.Until(t)

		if remains > updateFrequency {
			// If still have enough time, stop diligent scheduler.
			return
		} else if ytapi.IsLiveLiveBroadcast(v) {
			// If live already start, stop diligent scheduler & send notifies.
			notices, err := s.db.getNoticesByVideoID(v.Id)
			if err != nil {
				glog.Error(err)
				return
			}

			for _, n := range notices {
				// Remove record button
				go func() {
					cfg := tgbot.NewEditMessageReplyMarkup(n.chatID, n.messageID,
						tgbot.InlineKeyboardMarkup{InlineKeyboard: [][]tgbot.InlineKeyboardButton{{}}})
					s.tgSend(cfg)
				}()

				msgConfig := tgbot.NewMessage(n.chatID, fmt.Sprintf(
					"%s\n%s",
					tgbot.EscapeText(v.Snippet.ChannelTitle+" is now live!"),
					tgbot.InlineLink(
						tgbot.BordText(tgbot.EscapeText(v.Snippet.Title)),
						ytVideoURLPrefix+v.Id,
					),
				))
				msgConfig.DisableWebPagePreview = true

				s.tgSend(msgConfig)

				go func(n Notice) {
					time.Sleep(3 * time.Second)
					s.sendDownloadRequest(v, n)
				}(n)
			}

			return
		}

		time.Sleep(getWaitingDuration(remains))

		// WTF, scheduled start time has arrived but live still not started!
		if remains <= 0 {
			if (-remains)%30*time.Minute == 0 {
				glog.Warning("Running " + ytVideoURLPrefix + v.Id + " tolerance section")
				glog.Warning("Already " + (-remains).String() + " has elapsed")
			}

			// Well, lets wait for 30 more seconds.
			time.Sleep(30 * time.Second)
		}
	}
}

func getWaitingDuration(t time.Duration) time.Duration {
	var interval = [...]time.Duration{30 * time.Minute, 15 * time.Minute, 5 * time.Minute, 1 * time.Minute, 10 * time.Second, 0}

	for _, v := range interval {
		if t > v {
			return t - v
		}
	}

	return 0
}

func (s *Server) sendDownloadRequest(v *youtube.Video, n Notice) {
	eTitle := tgbot.EscapeText(v.Snippet.Title)
	vURL := ytVideoURLPrefix + v.Id

	var msgConfig tgbot.MessageConfig
	var internalServerError tgbot.MessageConfig = tgbot.NewMessage(
		n.chatID,
		fmt.Sprintf("Record %s failed, internal server error", tgbot.InlineLink(eTitle, vURL)),
	)

	defer func() {
		if msgConfig != (tgbot.MessageConfig{}) {
			msgConfig.DisableNotification = true
			msgConfig.DisableWebPagePreview = true
			s.tgSend(msgConfig)
		}
	}()

	// Check record existence
	var exists bool
	err := s.db.QueryRow(
		"SELECT EXISTS(SELECT * FROM records WHERE chatID = ? and videoID = ?);",
		n.chatID, n.videoID,
	).Scan(&exists)

	if err != nil && err != sql.ErrNoRows {
		glog.Error(err)
		msgConfig = internalServerError
	} else if exists {
		if r, ok := s.recorderTable[n.chatID]; ok {
			data := make(map[string]interface{})

			data["url"] = vURL
			data["platform"] = "YouTube"
			data["channelID"] = v.Snippet.ChannelId
			data["videoID"] = v.Id

			resp, err := r.Record(s.CallbackUrl()+"/recorder", data)

			if err != nil {
				if err.(*url.Error).Timeout() {
					msgConfig = tgbot.NewMessage(
						n.chatID,
						fmt.Sprintf("Record %s failed, connection timeout", tgbot.InlineLink(eTitle, vURL)),
					)
				} else {
					glog.Error(err)
					msgConfig = internalServerError
				}
			} else if resp.StatusCode != http.StatusOK {
				respBody, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					glog.Error(err)
					msgConfig = internalServerError
					return
				}

				fmt.Println(string(respBody))
				msgConfig = tgbot.NewMessage(
					n.chatID,
					fmt.Sprintf("Record request failed with status code %d, please check your recorder", resp.StatusCode),
				)
			} else {
				msgConfig = tgbot.NewMessage(
					n.chatID,
					fmt.Sprintf("Start recording %s", tgbot.InlineLink(tgbot.EscapeText(v.Snippet.Title), ytVideoURLPrefix+v.Id)),
				)
			}
		} else {
			msgConfig = tgbot.NewMessage(n.chatID, "Recorder unavailable for you")
		}
	}
}
