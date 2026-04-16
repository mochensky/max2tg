package src

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

type Database struct {
	db *sql.DB
}

func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	if err := createTables(db); err != nil {
		return nil, err
	}

	return &Database{db: db}, nil
}

func createTables(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			max_message_id INTEGER UNIQUE,
			tg_message_id INTEGER,
			max_sender_id INTEGER,
			timestamp INTEGER,
			edited_at INTEGER
		)
	`)
	return err
}

func (d *Database) AddMessage(maxMessageID, tgMessageID, maxSenderID, timestamp, editedAt int64) error {
	_, err := d.db.Exec(
		`INSERT OR REPLACE INTO messages (max_message_id, tg_message_id, max_sender_id, timestamp, edited_at) VALUES (?, ?, ?, ?, ?)`,
		maxMessageID, tgMessageID, maxSenderID, timestamp, editedAt,
	)
	return err
}

func (d *Database) GetMessageByMaxID(maxMessageID int64) (map[string]interface{}, error) {
	row := d.db.QueryRow(
		`SELECT id, max_message_id, tg_message_id, max_sender_id, timestamp, edited_at FROM messages WHERE max_message_id = ?`,
		maxMessageID,
	)

	var id, maxMsgID, tgMsgID, senderID, ts, editedAt int64
	err := row.Scan(&id, &maxMsgID, &tgMsgID, &senderID, &ts, &editedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return map[string]interface{}{
		"id":             id,
		"max_message_id": maxMsgID,
		"tg_message_id":  tgMsgID,
		"max_sender_id":  senderID,
		"timestamp":      ts,
		"edited_at":      editedAt,
	}, nil
}

func (d *Database) DeleteMessageByMaxID(maxMessageID int64) error {
	_, err := d.db.Exec(`DELETE FROM messages WHERE max_message_id = ?`, maxMessageID)
	return err
}

func (d *Database) UpdateMessageEditedAt(maxMessageID, editedAt int64) error {
	_, err := d.db.Exec(
		`UPDATE messages SET edited_at = ? WHERE max_message_id = ?`,
		editedAt, maxMessageID,
	)
	return err
}

func (d *Database) GetAllMessages() ([]map[string]interface{}, error) {
	rows, err := d.db.Query(
		`SELECT id, max_message_id, tg_message_id, max_sender_id, timestamp, edited_at FROM messages`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id, maxMsgID, tgMsgID, senderID, ts, editedAt int64
		if err := rows.Scan(&id, &maxMsgID, &tgMsgID, &senderID, &ts, &editedAt); err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"id":             id,
			"max_message_id": maxMsgID,
			"tg_message_id":  tgMsgID,
			"max_sender_id":  senderID,
			"timestamp":      ts,
			"edited_at":      editedAt,
		})
	}

	return results, nil
}

func (d *Database) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

func GetMoscowTime() time.Time {
	loc, _ := time.LoadLocation("Europe/Moscow")
	return time.Now().In(loc)
}
