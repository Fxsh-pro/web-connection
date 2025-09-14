package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Message struct {
	ID         int64  `json:"id"`
	Room       string `json:"room"`
	Sender     string `json:"sender"`
	SenderName string `json:"senderName"`
	Body       string `json:"body"`
	CreatedAt  string `json:"createdAt"`
}

func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, dsn)
}

func Migrate(db *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := db.Exec(ctx, `
        CREATE TABLE IF NOT EXISTS messages (
            id BIGSERIAL PRIMARY KEY,
            room TEXT NOT NULL,
            sender TEXT NOT NULL,
            body TEXT NOT NULL,
            created_at TIMESTAMPTZ NOT NULL DEFAULT now()
        );
        CREATE INDEX IF NOT EXISTS idx_messages_room_created ON messages(room, created_at DESC);
        ALTER TABLE messages ADD COLUMN IF NOT EXISTS sender_name TEXT;
    `)
	return err
}

func SaveMessage(ctx context.Context, db *pgxpool.Pool, room, sender, senderName, body, created string) (int64, error) {
	var id int64
	err := db.QueryRow(ctx, `
        INSERT INTO messages (room, sender, sender_name, body, created_at)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id
    `, room, sender, senderName, body, created).Scan(&id)
	return id, err
}

func LoadRecentMessages(ctx context.Context, db *pgxpool.Pool, room string, limit int) ([]Message, error) {
	rows, err := db.Query(ctx, `
        SELECT id, room, sender, COALESCE(sender_name, '') AS sender_name, body, to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SSZ')
        FROM messages
        WHERE room = $1
        ORDER BY created_at DESC
        LIMIT $2
    `, room, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Message, 0, limit)
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Room, &m.Sender, &m.SenderName, &m.Body, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}
