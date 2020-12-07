package hub

import (
	"bytes"
	"encoding/xml"
)

// Feed ...
type Feed struct {
	Entry        Entry        `xml:"entry"`
	DeletedEntry DeletedEntry `xml:"http://purl.org/atompub/tombstones/1.0 deleted-entry"`
}

// Entry ...
type Entry struct {
	ID        string  `xml:"id"`
	VideoID   string  `xml:"http://www.youtube.com/xml/schemas/2015 videoId"`
	ChannelID string  `xml:"http://www.youtube.com/xml/schemas/2015 channelId"`
	Title     string  `xml:"title"`
	Link      *Link   `xml:"link"`
	Author    *Author `xml:"author"`
	Published string  `xml:"published"`
	Updated   string  `xml:"updated"`
}

// Link ...
type Link struct {
	Href string `xml:"href,attr"`
}

// Author ...
type Author struct {
	Name string `xml:"name"`
	URI  string `xml:"uri"`
}

// DeletedEntry ...
type DeletedEntry struct {
	Ref  string     `xml:"ref,attr"`
	When string     `xml:"when,attr"`
	Link *Link      `xml:"link"`
	By   RawMessage `xml:"http://purl.org/atompub/tombstones/1.0 by"`
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
