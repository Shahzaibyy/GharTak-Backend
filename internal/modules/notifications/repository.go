package notifications

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yourusername/ghartak-backend/internal/platform/database"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Tokens(ctx context.Context, accountID uuid.UUID) ([]string, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	rows, err := r.pool.Query(ctx, `SELECT token FROM device_tokens WHERE account_id = $1 LIMIT 5`, accountID)
	if err != nil {
		return nil, fmt.Errorf("notifications: tokens: %w", err)
	}
	defer rows.Close()
	return scanTokens(rows)
}

func (r *Repository) Save(ctx context.Context, accountID uuid.UUID, role, token string) error {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := r.pool.Exec(ctx, `
INSERT INTO device_tokens (account_id, role, token)
VALUES ($1, $2, $3)
ON CONFLICT (account_id, role, token) DO NOTHING`, accountID, role, token)
	if err != nil {
		return fmt.Errorf("notifications: save token: %w", err)
	}
	return nil
}

func scanTokens(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]string, error) {
	out := make([]string, 0)
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			return nil, fmt.Errorf("notifications: tokens: %w", err)
		}
		out = append(out, token)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("notifications: tokens: %w", err)
	}
	return out, nil
}
