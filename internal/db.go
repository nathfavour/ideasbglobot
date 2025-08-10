package internal

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/nathfavour/ideasbglobot/internal/config"
)

var DB *sql.DB

type Message struct {
	ID       int64
	ChatID   int64
	UserID   int64
	Username string
	Text     string
	IsBot    bool
	Type     string
	Created  time.Time
}

func EnsureDatabase() error {
	dbPath := filepath.Join(config.GetAppDir(), "data.db")
	dir := filepath.Dir(dbPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}
	return InitDatabase()
}

func InitDatabase() error {
	dbPath := filepath.Join(config.GetAppDir(), "data.db")
	var err error
	DB, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			chat_id INTEGER,
			user_id INTEGER,
			username TEXT,
			text TEXT,
			is_bot BOOLEAN,
			type TEXT,
			created DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}
	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS auto_replies (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			category TEXT,
			reply TEXT,
			context TEXT
		)
	`)
	return err
}

func SaveMessage(msg Message) error {
	_, err := DB.Exec(`
		INSERT INTO messages (chat_id, user_id, username, text, is_bot, type, created) 
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		msg.ChatID, msg.UserID, msg.Username, msg.Text, msg.IsBot, msg.Type, msg.Created)
	return err
}
