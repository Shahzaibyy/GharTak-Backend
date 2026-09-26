package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/database"
)

const upsertCustomerSQL = `
WITH inserted AS (
    INSERT INTO users (phone_ciphertext, phone_lookup)
    VALUES ($1, $2)
    ON CONFLICT (phone_lookup) DO NOTHING
    RETURNING id, status
)
SELECT id, status FROM inserted
UNION ALL
SELECT id, status FROM users
WHERE phone_lookup = $2 AND NOT EXISTS (SELECT 1 FROM inserted)`

var findAccountSQL = map[Role]string{
	RoleRider: `
SELECT id, verification_status
FROM riders
WHERE phone_lookup = $1`,
	RoleMerchant: `
SELECT id, verification_status
FROM merchants
WHERE phone_lookup = $1`,
	RoleAdmin: `
SELECT id, status
FROM admins
WHERE phone_lookup = $1`,
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) UpsertCustomer(ctx context.Context, ciphertext, lookup string) (Account, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var account Account
	err := r.pool.QueryRow(ctx, upsertCustomerSQL, ciphertext, lookup).Scan(&account.ID, &account.Status)
	if err != nil {
		return Account{}, fmt.Errorf("auth: upsert customer: %w", err)
	}
	return account, nil
}

func (r *Repository) Find(ctx context.Context, role Role, lookup string) (Account, error) {
	query, ok := findAccountSQL[role]
	if !ok {
		return Account{}, apperror.ErrUnauthorized
	}
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var account Account
	err := r.pool.QueryRow(ctx, query, lookup).Scan(&account.ID, &account.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, apperror.ErrNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("auth: find account: %w", err)
	}
	return account, nil
}
