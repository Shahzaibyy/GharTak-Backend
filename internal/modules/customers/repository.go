package customers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/database"
)

const insertAddressSQL = `
INSERT INTO addresses (user_id, label, lat, lng, address_text)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, user_id, label, lat, lng, address_text, created_at, updated_at`

const listAddressSQL = `
SELECT id, user_id, label, lat, lng, address_text, created_at, updated_at
FROM addresses
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT 20`

const updateAddressSQL = `
UPDATE addresses
SET label = $3, lat = $4, lng = $5, address_text = $6, updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING id, user_id, label, lat, lng, address_text, created_at, updated_at`

const deleteAddressSQL = `
DELETE FROM addresses
WHERE id = $1 AND user_id = $2`

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Insert(ctx context.Context, userID uuid.UUID, in AddressInput) (Address, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	address, err := scanAddress(r.pool.QueryRow(ctx, insertAddressSQL, userID, in.Label, in.Lat, in.Lng, in.AddressText))
	if err != nil {
		return Address{}, fmt.Errorf("customers: insert address: %w", err)
	}
	return address, nil
}

func (r *Repository) List(ctx context.Context, userID uuid.UUID) ([]Address, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	rows, err := r.pool.Query(ctx, listAddressSQL, userID)
	if err != nil {
		return nil, fmt.Errorf("customers: list addresses: %w", err)
	}
	defer rows.Close()
	return scanAddresses(rows)
}

func (r *Repository) Update(ctx context.Context, userID, addressID uuid.UUID, in AddressInput) (Address, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	address, err := scanAddress(r.pool.QueryRow(ctx, updateAddressSQL, addressID, userID, in.Label, in.Lat, in.Lng, in.AddressText))
	if errors.Is(err, pgx.ErrNoRows) {
		return Address{}, apperror.ErrNotFound
	}
	if err != nil {
		return Address{}, fmt.Errorf("customers: update address: %w", err)
	}
	return address, nil
}

func (r *Repository) Delete(ctx context.Context, userID, addressID uuid.UUID) error {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tag, err := r.pool.Exec(ctx, deleteAddressSQL, addressID, userID)
	if err != nil {
		return fmt.Errorf("customers: delete address: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

func scanAddresses(rows pgx.Rows) ([]Address, error) {
	out := make([]Address, 0)
	for rows.Next() {
		address, err := scanAddress(rows)
		if err != nil {
			return nil, fmt.Errorf("customers: list addresses: %w", err)
		}
		out = append(out, address)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("customers: list addresses: %w", err)
	}
	return out, nil
}

func scanAddress(row pgx.Row) (Address, error) {
	var address Address
	err := row.Scan(
		&address.ID, &address.UserID, &address.Label, &address.Lat, &address.Lng,
		&address.AddressText, &address.CreatedAt, &address.UpdatedAt,
	)
	if err != nil {
		return Address{}, err
	}
	return address, nil
}
