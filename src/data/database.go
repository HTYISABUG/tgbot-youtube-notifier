package data

import (
	"database/sql"
	"tgbot"

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

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS channels (id TEXT PRIMARY KEY);")
	if err != nil {
		return nil, err
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, chat_id INTEGER);")
	if err != nil {
		return nil, err
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS subscribers (user_id INTEGER, channel_id TEXT, PRIMARY KEY (user_id, channel_id));")
	if err != nil {
		return nil, err
	}

	return &Database{DB: db}, nil
}

// Subscribe registers info into corresponding table
func (db *Database) Subscribe(info tgbot.SubscribeInfo) error {
	stmt, err := db.Prepare("INSERT OR IGNORE INTO channels (id) VALUES (?);")
	if err != nil {
		return err
	}

	_, err = stmt.Exec(info.ChannelID)
	if err != nil {
		return err
	}

	stmt, err = db.Prepare("INSERT OR IGNORE INTO users (id, chat_id) VALUES (?, ?);")
	if err != nil {
		return err
	}

	_, err = stmt.Exec(info.UserID, info.ChatID)
	if err != nil {
		return err
	}

	stmt, err = db.Prepare("INSERT OR IGNORE INTO subscribers (user_id, channel_id) VALUES (?, ?);")
	if err != nil {
		return err
	}

	_, err = stmt.Exec(info.UserID, info.ChannelID)
	if err != nil {
		return err
	}

	return nil
}
