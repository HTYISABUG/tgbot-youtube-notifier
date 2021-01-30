package server

import (
	"database/sql"
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
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS channels (id VARCHAR(255) PRIMARY KEY, title TEXT);")
	if err != nil {
		return nil, err
	}

	// Create table to save subscribing chat data
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS chats (id INT PRIMARY KEY, admin BOOL DEFAULT 0);")
	if err != nil {
		return nil, err
	}

	// Create table to save subscribers data pairs
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS subscribers (" +
		"chatID INT, channelID VARCHAR(255), PRIMARY KEY (chatID, channelID));")
	if err != nil {
		return nil, err
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS monitoring (" +
		"videoID VARCHAR(255), chatID INT, messageID INT, PRIMARY KEY (videoID, chatID));")
	if err != nil {
		return nil, err
	}

	// Create table to save videos status
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS videos (id VARCHAR(255) PRIMARY KEY, completed BOOL);")
	if err != nil {
		return nil, err
	}

	return &database{DB: db}, nil
}

// Subscribe registers info into corresponding table
func (db *database) subscribe(chat rowChat, channel rowChannel) error {
	_, err := db.Exec("INSERT IGNORE INTO chats (id, admin) VALUES (?, ?);", chat.id, false)
	if err != nil {
		return err
	}

	_, err = db.Exec("INSERT IGNORE INTO channels (id, title) VALUES (?, ?);", channel.id, channel.title)
	if err != nil {
		return err
	}

	_, err = db.Exec("INSERT IGNORE INTO subscribers (chatID, channelID) VALUES (?, ?);", chat.id, channel.id)
	if err != nil {
		return err
	}

	return nil
}

func (db *database) getSubscribeChatsByChannelID(channelID string) ([]rowChat, error) {
	rows, err := db.Query(
		"SELECT chats.id, chats.admin FROM "+
			"chats INNER JOIN subscribers ON chats.id = subscribers.chatID "+
			"WHERE subscribers.channelID = ?;",
		channelID,
	)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var results []rowChat
	var chat rowChat
	for rows.Next() {
		err := rows.Scan(&chat.id, &chat.admin)
		if err != nil {
			return nil, err
		}

		results = append(results, chat)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return results, nil
}

func (db *database) getSubscribedChannels() ([]rowChannel, error) {
	var rows *sql.Rows
	var err error

	rows, err = db.Query("SELECT * FROM channels;")
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var results []rowChannel
	var channel rowChannel
	for rows.Next() {
		err := rows.Scan(&channel.id, &channel.title)
		if err != nil {
			return nil, err
		}

		results = append(results, channel)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return results, nil
}

func (db *database) getSubscribedChannelsByChatID(chatID int64) ([]rowChannel, error) {
	rows, err := db.Query(
		"SELECT channels.id, channels.title FROM "+
			"channels INNER JOIN subscribers ON channels.id = subscribers.channelID "+
			"WHERE subscribers.chatID = ?;",
		chatID,
	)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var results []rowChannel
	var channel rowChannel

	for rows.Next() {
		err := rows.Scan(&channel.id, &channel.title)
		if err != nil {
			return nil, err
		}

		results = append(results, channel)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return results, nil
}

func (db *database) getMonitoringMessagesByVideoID(videoID string) ([]rowMonitoring, error) {
	rows, err := db.Query(
		"SELECT videoID, chatID, messageID FROM monitoring WHERE videoID = ?;",
		videoID,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var results []rowMonitoring
	var monitoring rowMonitoring
	for rows.Next() {
		err := rows.Scan(&monitoring.videoID, &monitoring.chatID, &monitoring.messageID)
		if err != nil {
			return nil, err
		}

		results = append(results, monitoring)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return results, nil
}
