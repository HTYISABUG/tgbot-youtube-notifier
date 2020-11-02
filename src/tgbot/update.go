package tgbot

type update struct {
	ID      int      `json:"update_id"`
	Message *message `json:"message"`
}

type message struct {
	ID       int             `json:"messade_id"`
	Date     int             `json:"date"`
	From     *user           `json:"from"`
	Chat     *chat           `json:"chat"`
	Text     *string         `json:"text"`
	Entities []messageEntity `json:"entities"`
}

type user struct {
	ID    int   `json:"id"`
	IsBot *bool `json:"is_bot"`
}

type chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

type messageEntity struct {
	Type   string  `json:"type"`
	Offset int     `json:"offset"`
	Length int     `json:"length"`
	URL    *string `json:"url"`
}
