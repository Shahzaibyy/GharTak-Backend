package chat

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yourusername/ghartak-backend/internal/platform/database"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Insert(ctx context.Context, orderID uuid.UUID, role string, senderID uuid.UUID, body string) (Message, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var msg Message
	err := r.pool.QueryRow(ctx, `
INSERT INTO chat_messages (order_id, sender_role, sender_id, body)
VALUES ($1, $2, $3, $4)
RETURNING id, order_id, sender_role, sender_id, body, created_at`, orderID, role, senderID, body).Scan(
		&msg.ID, &msg.OrderID, &msg.SenderRole, &msg.SenderID, &msg.Body, &msg.CreatedAt,
	)
	if err != nil {
		return Message{}, fmt.Errorf("chat: insert: %w", err)
	}
	return msg, nil
}

func (r *Repository) List(ctx context.Context, orderID uuid.UUID) ([]Message, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	rows, err := r.pool.Query(ctx, `
SELECT id, order_id, sender_role, sender_id, body, created_at
FROM chat_messages
WHERE order_id = $1
ORDER BY created_at DESC
LIMIT 50`, orderID)
	if err != nil {
		return nil, fmt.Errorf("chat: list: %w", err)
	}
	defer rows.Close()
	return scanMessages(rows)
}

func scanMessages(rows pgx.Rows) ([]Message, error) {
	out := make([]Message, 0)
	for rows.Next() {
		msg, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chat: list: %w", err)
	}
	return out, nil
}

func scanMessage(row pgx.Row) (Message, error) {
	var msg Message
	err := row.Scan(&msg.ID, &msg.OrderID, &msg.SenderRole, &msg.SenderID, &msg.Body, &msg.CreatedAt)
	if err != nil {
		return Message{}, fmt.Errorf("chat: list: %w", err)
	}
	return msg, nil
}
