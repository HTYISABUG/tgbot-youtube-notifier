package hub

import (
	"bytes"
	"encoding/xml"
)

type feed struct {
	Entry Entry `xml:"entry"`
}

// Entry is for containing entry element in Feed.
type Entry struct {
	ID        string     `xml:"id"`
	VideoID   string     `xml:"http://www.youtube.com/xml/schemas/2015 videoId"`
	ChannelID string     `xml:"http://www.youtube.com/xml/schemas/2015 channelId"`
	Title     string     `xml:"title"`
	Link      RawMessage `xml:"link"`
	Author    *Author    `xml:"author"`
	Published string     `xml:"published"`
	Updated   string     `xml:"updated"`
}

// Author ...
type Author struct {
	Name string `xml:"name"`
	URI  string `xml:"uri"`
}

// RawMessage ...
type RawMessage []byte

// UnmarshalXML ...
func (m *RawMessage) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var b bytes.Buffer
	e := xml.NewEncoder(&b)

	var field struct {
		InnerXML []byte `xml:",innerxml"`
	}

	if err := d.DecodeElement(&field, &start); err != nil {
		return err
	}

	e.EncodeElement(field, start)

	*m = append((*m)[0:0], b.Bytes()...)

	return nil
}
