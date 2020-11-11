package data

import (
	"database/sql"
	"info"

	_ "github.com/mattn/go-sqlite3" // SQLite3 driver
)

// Database manages all data that server needs to save
type Database struct {
	*sql.DB
}

// NewDatabase returns a pointer to a new `Database` object.
func NewDatabase(dataSourceName string) (*Database, error) {
	db, err := sql.Open("sqlite3", dataSourceName)
	if err != nil {
		return nil, err
	}

	// Create table to save subscribed channel data
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS channels (id TEXT PRIMARY KEY, title TEXT);")
	if err != nil {
		return nil, err
	}

	// Create table to save subscribing user data
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, chat_id INTEGER);")
	if err != nil {
		return nil, err
	}

	// Create table to save subscribers data
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS subscribers (" +
		"user_id INTEGER, channel_id TEXT, PRIMARY KEY (user_id, channel_id));")
	if err != nil {
		return nil, err
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS notifications (" +
		"video_id TEXT, chat_id INTEGER, message_id INTEGER, PRIMARY KEY (video_id, chat_id));")
	if err != nil {
		return nil, err
	}

	return &Database{DB: db}, nil
}

// Subscribe registers info into corresponding table
func (db *Database) Subscribe(subInfo info.SubscribeInfo, chInfo info.ChannelInfo) error {
	_, err := db.Exec("INSERT OR IGNORE INTO channels (id, title) VALUES (?, ?);", subInfo.ChannelID, chInfo.Title)
	if err != nil {
		return err
	}

	_, err = db.Exec("INSERT OR IGNORE INTO users (id, chat_id) VALUES (?, ?);", subInfo.UserID, subInfo.ChatID)
	if err != nil {
		return err
	}

	_, err = db.Exec("INSERT OR IGNORE INTO subscribers (user_id, channel_id) VALUES (?, ?);", subInfo.UserID, subInfo.ChannelID)
	if err != nil {
		return err
	}

	return nil
}

// GetSubsciberChatIDsByChannelID returns all users' chat id that subscribes the channel
func (db *Database) GetSubsciberChatIDsByChannelID(channelID string) ([]int64, error) {
	rows, err := db.Query("SELECT chat_id FROM users INNER JOIN subscribers ON users.id == subscribers.user_id WHERE subscribers.channel_id = ?;", channelID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var id int64
	var chatIDs []int64
	for rows.Next() {
		err := rows.Scan(&id)
		if err != nil {
			return nil, err
		}

		chatIDs = append(chatIDs, id)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return chatIDs, nil
}

// GetAllSubscibedChannelIDs returns all subscibed channels' id
func (db *Database) GetAllSubscibedChannelIDs() ([]string, error) {
	var rows *sql.Rows
	var err error

	rows, err = db.Query("SELECT id FROM channels;")
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var id string
	var chIDs []string
	for rows.Next() {
		err := rows.Scan(&id)
		if err != nil {
			return nil, err
		}

		chIDs = append(chIDs, id)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return chIDs, nil
}

// GetListInfosByUserID updates list infos by user id
func (db *Database) GetListInfosByUserID(info *info.ListInfo) error {
	var rows *sql.Rows
	var err error

	// rows, err = db.Query("SELECT title FROM subscribers INNER JOIN channels ON subscribers.channel_id = channels.id WHERE user_id = ?;", userID)
	rows, err = db.Query(
		"SELECT users.chat_id, channels.id, channels.title FROM "+
			"channels INNER JOIN (users INNER JOIN subscribers ON users.id = subscribers.user_id) "+
			"ON subscribers.channel_id = channels.id "+
			"WHERE subscribers.user_id = ?;",
		info.UserID,
	)
	if err != nil {
		return err
	}

	defer rows.Close()

	var chatID int64
	var chID, chTitle string
	for rows.Next() {
		err := rows.Scan(&chatID, &chID, &chTitle)
		if err != nil {
			return err
		}

		info.ChatID = chatID
		info.ChannelIDs = append(info.ChannelIDs, chID)
		info.ChannelTitles = append(info.ChannelTitles, chTitle)
	}

	if rows.Err() != nil {
		return rows.Err()
	}

	return nil
}

func (db *Database) Notify(info info.NotifyInfo, method string) error {
	_, err := db.Exec(
		"INSERT OR "+method+" INTO notifications (video_id, chat_id, message_id) VALUES (?, ?, ?);",
		info.VideoID, info.ChatID, info.MessageID,
	)

	if err != nil {
		return err
	}

	return nil
}

func (db *Database) GetNotifyInfosByVideoID(videoID string) ([]info.NotifyInfo, error) {
	rows, err := db.Query(
		"SELECT video_id, chat_id, message_id FROM notifications WHERE video_id = ?;",
		videoID,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var nInfo info.NotifyInfo
	var channels []info.NotifyInfo
	for rows.Next() {
		err := rows.Scan(&nInfo.VideoID, &nInfo.ChatID, &nInfo.MessageID)
		if err != nil {
			return nil, err
		}

		channels = append(channels, nInfo)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return channels, nil
}
