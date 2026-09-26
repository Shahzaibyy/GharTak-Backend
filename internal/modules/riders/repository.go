package riders

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
	"github.com/yourusername/ghartak-backend/internal/platform/domain"
)

const riderColumns = `
id, COALESCE(name, ''), verification_status, zone_id, COALESCE(vehicle_reg, ''), is_online,
rating::text, rating_count, cnic_object_key, selfie_object_key, license_object_key, created_at`

const insertRiderSQL = `
INSERT INTO riders (
    phone_ciphertext, phone_lookup, name, cnic_ciphertext, cnic_lookup,
    cnic_object_key, selfie_object_key, license_object_key, vehicle_reg, zone_id
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING ` + riderColumns

const getRiderSQL = `
SELECT ` + riderColumns + `
FROM riders
WHERE id = $1`

const listRidersSQL = `
SELECT ` + riderColumns + `
FROM riders
WHERE verification_status = $1
ORDER BY created_at
LIMIT 20`

const setOnlineSQL = `
UPDATE riders
SET is_online = $2, updated_at = now()
WHERE id = $1 AND ($2 = false OR (verification_status = 'approved' AND zone_id IS NOT NULL))
RETURNING ` + riderColumns

const approveRiderSQL = `
UPDATE riders
SET verification_status = 'approved', zone_id = $2, updated_at = now()
WHERE id = $1 AND verification_status = 'pending'
RETURNING ` + riderColumns

const rejectRiderSQL = `
UPDATE riders
SET verification_status = 'rejected', is_online = false, updated_at = now()
WHERE id = $1 AND verification_status = 'pending'
RETURNING ` + riderColumns

const suspendRiderSQL = `
UPDATE riders
SET verification_status = 'suspended', is_online = false, updated_at = now()
WHERE id = $1 AND verification_status = 'approved'
RETURNING ` + riderColumns

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Insert(ctx context.Context, row sealedRider) (Profile, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	profile, err := scanRider(r.pool.QueryRow(ctx, insertRiderSQL,
		row.PhoneCipher, row.PhoneLookup, row.Name, row.CNICCipher, row.CNICLookup,
		row.CNICObjectKey, row.SelfieObjectKey, row.LicenseObjectKey, row.VehicleReg, row.ZoneID,
	))
	if isUnique(err) {
		return Profile{}, apperror.ErrConflict
	}
	if err != nil {
		return Profile{}, fmt.Errorf("riders: insert: %w", err)
	}
	return profile, nil
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (Profile, error) {
	return r.one(ctx, getRiderSQL, id)
}

func (r *Repository) List(ctx context.Context, status domain.VerificationStatus) ([]Profile, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	rows, err := r.pool.Query(ctx, listRidersSQL, string(status))
	if err != nil {
		return nil, fmt.Errorf("riders: list: %w", err)
	}
	defer rows.Close()
	return scanRiders(rows)
}

func (r *Repository) SetOnline(ctx context.Context, id uuid.UUID, online bool) (Profile, error) {
	return r.one(ctx, setOnlineSQL, id, online)
}

func (r *Repository) Approve(ctx context.Context, id, zoneID uuid.UUID) (Profile, error) {
	return r.one(ctx, approveRiderSQL, id, zoneID)
}

func (r *Repository) Reject(ctx context.Context, id uuid.UUID) (Profile, error) {
	return r.one(ctx, rejectRiderSQL, id)
}

func (r *Repository) Suspend(ctx context.Context, id uuid.UUID) (Profile, error) {
	return r.one(ctx, suspendRiderSQL, id)
}

func (r *Repository) one(ctx context.Context, query string, args ...any) (Profile, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	profile, err := scanRider(r.pool.QueryRow(ctx, query, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, apperror.ErrNotFound
	}
	if err != nil {
		return Profile{}, fmt.Errorf("riders: query: %w", err)
	}
	return profile, nil
}

type sealedRider struct {
	PhoneCipher      string
	PhoneLookup      string
	Name             string
	CNICCipher       string
	CNICLookup       string
	CNICObjectKey    string
	SelfieObjectKey  string
	LicenseObjectKey string
	VehicleReg       string
	ZoneID           uuid.UUID
}

func scanRiders(rows pgx.Rows) ([]Profile, error) {
	out := make([]Profile, 0)
	for rows.Next() {
		profile, err := scanRider(rows)
		if err != nil {
			return nil, fmt.Errorf("riders: list: %w", err)
		}
		out = append(out, profile)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("riders: list: %w", err)
	}
	return out, nil
}

func scanRider(row pgx.Row) (Profile, error) {
	var profile Profile
	err := row.Scan(
		&profile.ID, &profile.Name, &profile.VerificationStatus, &profile.ZoneID, &profile.VehicleReg, &profile.IsOnline,
		&profile.Rating, &profile.RatingCount, &profile.CNICObjectKey, &profile.SelfieObjectKey, &profile.LicenseObjectKey, &profile.CreatedAt,
	)
	if err != nil {
		return Profile{}, err
	}
	return profile, nil
}

func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
