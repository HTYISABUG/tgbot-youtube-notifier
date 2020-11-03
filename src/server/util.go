package server

import (
	"fmt"
	"hub"
	"tgbot"
)

const ytVideoURLPrefix = "https://www.youtube.com/watch?v="

func entry2text(entry hub.Entry) string {
	return fmt.Sprintf(
		"%s\n%s",
		tgbot.InlineLink(
			tgbot.BordText(tgbot.Escape(entry.Title)),
			ytVideoURLPrefix+entry.VideoID,
		),
		tgbot.ItalicText(tgbot.Escape(entry.Author)),
	)
}
