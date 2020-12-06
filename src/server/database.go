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

	// Create table to save subscribing user data
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, chatID INTEGER);")
	if err != nil {
		return nil, err
	}

	// Create table to save subscribers data
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS subscribers (" +
		"userID INTEGER, channelID VARCHAR(255), PRIMARY KEY (userID, channelID));")
	if err != nil {
		return nil, err
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS monitoring (" +
		"videoID VARCHAR(255), chatID INTEGER, messageID INTEGER, PRIMARY KEY (videoID, chatID));")
	if err != nil {
		return nil, err
	}

	return &database{DB: db}, nil
}

// Subscribe registers info into corresponding table
func (db *database) subscribe(user rowUser, channel rowChannel) error {
	_, err := db.Exec("INSERT IGNORE INTO users (id, chatID) VALUES (?, ?);", user.id, user.chatID)
	if err != nil {
		return err
	}

	_, err = db.Exec("INSERT IGNORE INTO channels (id, title) VALUES (?, ?);", channel.id, channel.title)
	if err != nil {
		return err
	}

	_, err = db.Exec("INSERT IGNORE INTO subscribers (userID, channelID) VALUES (?, ?);", user.id, channel.id)
	if err != nil {
		return err
	}

	return nil
}

func (db *database) getSubscribeUsersByChannelID(channelID string) ([]rowUser, error) {
	rows, err := db.Query(
		"SELECT users.id, users.chatID FROM "+
			"users INNER JOIN subscribers ON users.id = subscribers.userID "+
			"WHERE subscribers.channelID = ?;",
		channelID,
	)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var results []rowUser
	var user rowUser
	for rows.Next() {
		err := rows.Scan(&user.id, &user.chatID)
		if err != nil {
			return nil, err
		}

		results = append(results, user)
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

func (db *database) getSubscribedChannelsByUserID(userID int) ([]rowChannel, error) {
	rows, err := db.Query(
		"SELECT channels.id, channels.title FROM "+
			"channels INNER JOIN subscribers ON channels.id = subscribers.channelID "+
			"WHERE subscribers.userID = ?;",
		userID,
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
