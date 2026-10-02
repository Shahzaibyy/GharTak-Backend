package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/database"
)

const upsertCustomerSQL = `
WITH inserted AS (
    INSERT INTO users (phone_ciphertext, phone_lookup, phone_verified)
    VALUES ($1, $2, true)
    ON CONFLICT (phone_lookup) WHERE phone_lookup IS NOT NULL DO NOTHING
    RETURNING id, status
)
SELECT id, status FROM inserted
UNION ALL
SELECT id, status FROM users
WHERE phone_lookup = $2 AND deleted_at IS NULL AND NOT EXISTS (SELECT 1 FROM inserted)`

const insertGoogleSQL = `
WITH inserted AS (
    INSERT INTO users (firebase_uid, email, name, phone_verified)
    VALUES ($1, $2, $3, false)
    ON CONFLICT (firebase_uid) WHERE firebase_uid IS NOT NULL DO NOTHING
    RETURNING id, status
)
SELECT id, status FROM inserted
UNION ALL
SELECT id, status FROM users
WHERE firebase_uid = $1 AND deleted_at IS NULL AND NOT EXISTS (SELECT 1 FROM inserted)`

const findFirebaseSQL = `
SELECT id, status
FROM users
WHERE firebase_uid = $1 AND deleted_at IS NULL`

const profileSQL = `
SELECT id, name, email, phone_ciphertext, phone_verified, wallet_balance::text, preferred_order_types
FROM users
WHERE id = $1 AND deleted_at IS NULL`

const updateNameSQL = `
UPDATE users
SET name = $2, updated_at = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, name, email, phone_ciphertext, phone_verified, wallet_balance::text, preferred_order_types`

const setPreferencesSQL = `
UPDATE users
SET preferred_order_types = $2, updated_at = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, name, email, phone_ciphertext, phone_verified, wallet_balance::text, preferred_order_types`

const findEmailSQL = `
SELECT id, status
FROM users
WHERE lower(email) = $1 AND deleted_at IS NULL`

const insertEmailSQL = `
WITH inserted AS (
    INSERT INTO users (email, phone_verified)
    VALUES ($1, false)
    ON CONFLICT DO NOTHING
    RETURNING id, status
)
SELECT id, status FROM inserted
UNION ALL
SELECT id, status FROM users
WHERE lower(email) = $1 AND deleted_at IS NULL AND NOT EXISTS (SELECT 1 FROM inserted)`

const attachPhoneSQL = `
UPDATE users
SET phone_ciphertext = $2, phone_lookup = $3, phone_verified = true, updated_at = now()
WHERE id = $1 AND deleted_at IS NULL`

const phoneVerifiedSQL = `
SELECT phone_verified
FROM users
WHERE id = $1 AND deleted_at IS NULL`

const lockUserSQL = `
SELECT deleted_at IS NOT NULL, wallet_balance = 0
FROM users
WHERE id = $1
FOR UPDATE`

const scrubUserSQL = `
UPDATE users
SET name = 'Deleted',
    email = NULL,
    phone_ciphertext = NULL,
    phone_lookup = NULL,
    firebase_uid = NULL,
    phone_verified = false,
    deleted_at = now(),
    updated_at = now()
WHERE id = $1 AND deleted_at IS NULL`

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

func (r *Repository) FindByFirebase(ctx context.Context, uid string) (Account, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var account Account
	err := r.pool.QueryRow(ctx, findFirebaseSQL, uid).Scan(&account.ID, &account.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, apperror.ErrNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("auth: find firebase user: %w", err)
	}
	return account, nil
}

func (r *Repository) FindByEmail(ctx context.Context, email string) (Account, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var account Account
	err := r.pool.QueryRow(ctx, findEmailSQL, email).Scan(&account.ID, &account.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, apperror.ErrNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("auth: find email user: %w", err)
	}
	return account, nil
}

func (r *Repository) InsertEmail(ctx context.Context, email string) (Account, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var account Account
	err := r.pool.QueryRow(ctx, insertEmailSQL, email).Scan(&account.ID, &account.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, apperror.ErrConflict
	}
	if err != nil {
		return Account{}, fmt.Errorf("auth: insert email user: %w", err)
	}
	return account, nil
}

func (r *Repository) InsertGoogle(ctx context.Context, row googleInsert) (Account, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var account Account
	err := r.pool.QueryRow(ctx, insertGoogleSQL, row.UID, row.Email, row.Name).Scan(&account.ID, &account.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, apperror.ErrConflict
	}
	if err != nil {
		return Account{}, fmt.Errorf("auth: insert google user: %w", err)
	}
	return account, nil
}

func (r *Repository) Profile(ctx context.Context, id uuid.UUID) (profileRow, error) {
	return r.oneProfile(ctx, profileSQL, id)
}

func (r *Repository) UpdateName(ctx context.Context, id uuid.UUID, name string) (profileRow, error) {
	return r.oneProfile(ctx, updateNameSQL, id, name)
}

func (r *Repository) SetPreferences(ctx context.Context, id uuid.UUID, types []string) (profileRow, error) {
	return r.oneProfile(ctx, setPreferencesSQL, id, types)
}

func (r *Repository) oneProfile(ctx context.Context, query string, args ...any) (profileRow, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	row, err := scanProfile(r.pool.QueryRow(ctx, query, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return profileRow{}, apperror.ErrNotFound
	}
	if err != nil {
		return profileRow{}, fmt.Errorf("auth: profile: %w", err)
	}
	return row, nil
}

func (r *Repository) AttachPhone(ctx context.Context, id uuid.UUID, ciphertext, lookup string) error {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tag, err := r.pool.Exec(ctx, attachPhoneSQL, id, ciphertext, lookup)
	if isUnique(err) {
		return apperror.Conflict("phone is already in use")
	}
	if err != nil {
		return fmt.Errorf("auth: attach phone: %w", err)
	}
	return missingUpdate(tag)
}

func (r *Repository) PhoneVerified(ctx context.Context, id uuid.UUID) (bool, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var verified bool
	err := r.pool.QueryRow(ctx, phoneVerifiedSQL, id).Scan(&verified)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, apperror.ErrNotFound
	}
	if err != nil {
		return false, fmt.Errorf("auth: phone verified: %w", err)
	}
	return verified, nil
}

func (r *Repository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("auth: begin delete: %w", err)
	}
	if err := deleteAccount(ctx, tx, id); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("auth: commit delete: %w", err)
	}
	return nil
}

func deleteAccount(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	state, err := lockUser(ctx, tx, id)
	if err != nil {
		return err
	}
	if err := canDelete(state); err != nil {
		return err
	}
	return scrubUser(ctx, tx, id)
}

func lockUser(ctx context.Context, tx pgx.Tx, id uuid.UUID) (userLock, error) {
	var state userLock
	err := tx.QueryRow(ctx, lockUserSQL, id).Scan(&state.deleted, &state.zero)
	if errors.Is(err, pgx.ErrNoRows) {
		return userLock{}, apperror.ErrNotFound
	}
	if err != nil {
		return userLock{}, fmt.Errorf("auth: lock user: %w", err)
	}
	return state, nil
}

func canDelete(state userLock) error {
	if state.deleted {
		return apperror.ErrNotFound
	}
	if !state.zero {
		return apperror.Conflict("wallet balance must be zero")
	}
	return nil
}

func scrubUser(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	tag, err := tx.Exec(ctx, scrubUserSQL, id)
	if err != nil {
		return fmt.Errorf("auth: scrub user: %w", err)
	}
	return missingUpdate(tag)
}

func scanProfile(row pgx.Row) (profileRow, error) {
	var found profileRow
	err := row.Scan(
		&found.ID, &found.Name, &found.Email, &found.PhoneCipher, &found.PhoneVerified, &found.Wallet,
		&found.PreferredOrderTypes,
	)
	if err != nil {
		return profileRow{}, err
	}
	return found, nil
}

func missingUpdate(tag pgconn.CommandTag) error {
	if tag.RowsAffected() == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
