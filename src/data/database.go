package data

import (
	"database/sql"
	"tgbot"
	"ytapi"

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
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS subscribers (user_id INTEGER, channel_id TEXT, PRIMARY KEY (user_id, channel_id));")
	if err != nil {
		return nil, err
	}

	return &Database{DB: db}, nil
}

// Subscribe registers info into corresponding table
func (db *Database) Subscribe(subInfo tgbot.SubscribeInfo, chInfo ytapi.ChannelInfo) error {
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

// GetSubsciberChats returns all user chat_id that subscribes the channel
func (db *Database) GetSubsciberChats(channelID string) ([]int64, error) {
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

// GetSubscibedChannels returns all subscribed channels
func (db *Database) GetSubscibedChannels() ([]string, error) {
	rows, err := db.Query("SELECT id FROM channels;")
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var id string
	var channels []string
	for rows.Next() {
		err := rows.Scan(&id)
		if err != nil {
			return nil, err
		}

		channels = append(channels, id)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return channels, nil
}
