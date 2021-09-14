package server

import (
	"database/sql"
	"reflect"
	"time"

	_ "github.com/go-sql-driver/mysql" // mysql driver
)

type database struct {
	*sql.DB
}

func newDatabase(dataSourceName string) (*database, error) {
	db, err := sql.Open("mysql", dataSourceName)
	if err != nil {
		return nil, err
	}

	db.SetConnMaxLifetime(4 * time.Minute)
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(8)

	// Create table to save subscribed channel data
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS channels (" +
		"id VARCHAR(255) PRIMARY KEY, title TEXT);")
	if err != nil {
		return nil, err
	}

	// Create table to save subscribing chat data
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS chats (" +
		"id BIGINT PRIMARY KEY, recorder TEXT, token TEXT);")
	if err != nil {
		return nil, err
	}

	// Create table to save subscribers data pairs
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS subscribers (" +
		"chatID BIGINT, channelID VARCHAR(255), PRIMARY KEY (chatID, channelID));")
	if err != nil {
		return nil, err
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS notices (" +
		"videoID VARCHAR(255), chatID BIGINT, messageID INT, PRIMARY KEY (videoID, chatID));")
	if err != nil {
		return nil, err
	}

	// Create table to save videos status
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS videos (" +
		"id VARCHAR(255) PRIMARY KEY, channelID TEXT, title TEXT, startTime BIGINT, completed BOOL);")
	if err != nil {
		return nil, err
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS filters (" +
		"chatID BIGINT, channelID VARCHAR(255), block BOOL, content TEXT, PRIMARY KEY (chatID, channelID, block));")
	if err != nil {
		return nil, err
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS autorecords (" +
		"chatID BIGINT, channelID VARCHAR(255), PRIMARY KEY (chatID, channelID));")
	if err != nil {
		return nil, err
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS records (" +
		"chatID BIGINT, videoID VARCHAR(255), done BOOL, PRIMARY KEY (chatID, videoID));")
	if err != nil {
		return nil, err
	}

	return &database{DB: db}, nil
}

// Subscribe registers info into corresponding table
func (db *database) subscribe(chatID int64, channel Channel) error {
	_, err := db.Exec("INSERT IGNORE INTO chats (id) VALUES (?);", chatID)
	if err != nil {
		return err
	}

	_, err = db.Exec("INSERT IGNORE INTO channels (id, title) VALUES (?, ?);", channel.id, channel.title)
	if err != nil {
		return err
	}

	_, err = db.Exec("INSERT IGNORE INTO subscribers (chatID, channelID) VALUES (?, ?);", chatID, channel.id)
	if err != nil {
		return err
	}

	return nil
}

func (db *database) getChannels() ([]Channel, error) {
	var channels []Channel

	err := db.queryResults(
		&channels,
		func(rows *sql.Rows, dest interface{}) error {
			r := dest.(*Channel)
			return rows.Scan(&r.id, &r.title)
		},
		"SELECT id, title FROM channels;",
	)

	if err != nil {
		return nil, err
	}

	return channels, nil
}

func (db *database) getChannelTitle(channelID string) (string, error) {
	var title string

	err := db.QueryRow("SELECT title FROM channels WHERE id = ?;", channelID).Scan(&title)
	if err != nil {
		return "", err
	}

	return title, nil
}

func (db *database) getChannelsByChatID(chatID int64) ([]Channel, error) {
	var results []Channel

	err := db.queryResults(
		&results,
		func(rows *sql.Rows, dest interface{}) error {
			ch := dest.(*Channel)
			return rows.Scan(&ch.id, &ch.title)
		},
		"SELECT channels.id, channels.title FROM "+
			"channels INNER JOIN subscribers ON channels.id = subscribers.channelID "+
			"WHERE subscribers.chatID = ?;",
		chatID,
	)

	if err != nil {
		return nil, err
	}

	return results, nil
}

func (db *database) getNoticesByVideoID(videoID string) ([]Notice, error) {
	var results []Notice

	err := db.queryResults(
		&results,
		func(rows *sql.Rows, dest interface{}) error {
			r := dest.(*Notice)
			return rows.Scan(&r.videoID, &r.chatID, &r.messageID)
		},
		"SELECT videoID, chatID, messageID FROM notices WHERE videoID = ?;",
		videoID,
	)

	if err != nil {
		return nil, err
	}

	return results, nil
}

func (db *database) getChatsByChannelID(channelID string) ([]Chat, error) {
	var results []Chat

	err := db.queryResults(
		&results,
		func(rows *sql.Rows, dest interface{}) error {
			r := dest.(*Chat)
			return rows.Scan(&r.id, &r.recorder, &r.token)
		},
		"SELECT id, recorder, token FROM "+
			"chats INNER JOIN subscribers ON chats.id = subscribers.chatID "+
			"WHERE subscribers.channelID = ?;",
		channelID,
	)

	if err != nil {
		return nil, err
	}

	return results, nil
}

func (db *database) queryResults(
	container interface{},
	scan func(rows *sql.Rows, dest interface{}) error,
	query string,
	args ...interface{},
) error {

	rows, err := db.Query(query, args...)
	if err != nil {
		return err
	}

	defer rows.Close()

	results := reflect.ValueOf(container).Elem()
	element := reflect.New(results.Type().Elem())
	for rows.Next() {
		if err := scan(rows, element.Interface()); err != nil {
			return err
		}

		results.Set(reflect.Append(results, element.Elem()))
	}

	if rows.Err() != nil {
		return rows.Err()
	}

	return nil
}
