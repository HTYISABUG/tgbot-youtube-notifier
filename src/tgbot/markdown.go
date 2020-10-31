package tgbot

import "fmt"

// BordText transforms text into telegram bord text.
func BordText(text string) string {
	return fmt.Sprintf("*%s*", text)
}

// InlineLink combines text and link into telegram inline link.
func InlineLink(text, link string) string {
	return fmt.Sprintf("[%s](%s)", text, link)
}

// ItalicText transforms text into telegram italic text.
func ItalicText(text string) string {
	return fmt.Sprintf("_%s_", text)
}
