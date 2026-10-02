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
id, COALESCE(name, ''), verification_status, zone_id, COALESCE(vehicle_reg, ''), vehicle_type,
license_number, is_online, rating::text, rating_count,
cnic_object_key, cnic_front_object_key, cnic_back_object_key, selfie_object_key, license_object_key,
orientation_status, orientation_preferred_slot, application_received_at, onboarding_step, created_at`

const insertRiderSQL = `
INSERT INTO riders (
    phone_ciphertext, phone_lookup, name, cnic_ciphertext, cnic_lookup,
    cnic_object_key, cnic_front_object_key, selfie_object_key, license_object_key,
    vehicle_reg, zone_id, onboarding_step, application_received_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING ` + riderColumns

const insertShellSQL = `
INSERT INTO riders (phone_ciphertext, phone_lookup, zone_id, onboarding_step)
VALUES ($1, $2, $3, 'applied')
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
WHERE id = $1 AND (
    $2 = false OR (
        verification_status = 'approved'
        AND zone_id IS NOT NULL
        AND orientation_status IN ('booked', 'completed')
    )
)
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

const updateDetailsSQL = `
UPDATE riders
SET name = $2, cnic_ciphertext = $3, cnic_lookup = $4, vehicle_type = $5,
    vehicle_reg = $6, license_number = $7, onboarding_step = 'details', updated_at = now()
WHERE id = $1 AND verification_status = 'pending'
RETURNING ` + riderColumns

const updateDocumentsSQL = `
UPDATE riders
SET cnic_front_object_key = $2, cnic_back_object_key = $3, cnic_object_key = $2,
    license_object_key = $4, selfie_object_key = $5, onboarding_step = 'documents', updated_at = now()
WHERE id = $1 AND verification_status = 'pending'
RETURNING ` + riderColumns

const submitSQL = `
UPDATE riders
SET onboarding_step = 'submitted', application_received_at = COALESCE(application_received_at, now()), updated_at = now()
WHERE id = $1 AND verification_status = 'pending'
  AND name IS NOT NULL AND cnic_lookup IS NOT NULL
  AND cnic_front_object_key IS NOT NULL AND cnic_back_object_key IS NOT NULL
  AND license_object_key IS NOT NULL AND selfie_object_key IS NOT NULL
  AND vehicle_type IS NOT NULL
RETURNING ` + riderColumns

const bookOrientationSQL = `
UPDATE riders
SET orientation_status = 'booked', orientation_preferred_slot = $2, updated_at = now()
WHERE id = $1 AND verification_status IN ('pending', 'approved')
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
	now := time.Now().UTC()
	profile, err := scanRider(r.pool.QueryRow(ctx, insertRiderSQL,
		row.PhoneCipher, row.PhoneLookup, row.Name, row.CNICCipher, row.CNICLookup,
		row.CNICObjectKey, row.CNICObjectKey, row.SelfieObjectKey, row.LicenseObjectKey,
		row.VehicleReg, row.ZoneID, string(domain.OnboardingSubmitted), now,
	))
	if isUnique(err) {
		return Profile{}, apperror.ErrConflict
	}
	if err != nil {
		return Profile{}, fmt.Errorf("riders: insert: %w", err)
	}
	return profile, nil
}

func (r *Repository) InsertShell(ctx context.Context, phoneCipher, phoneLookup string, zoneID uuid.UUID) (Profile, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	profile, err := scanRider(r.pool.QueryRow(ctx, insertShellSQL, phoneCipher, phoneLookup, zoneID))
	if isUnique(err) {
		return Profile{}, apperror.ErrConflict
	}
	if err != nil {
		return Profile{}, fmt.Errorf("riders: insert shell: %w", err)
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

func (r *Repository) UpdateDetails(ctx context.Context, id uuid.UUID, row detailsRow) (Profile, error) {
	return r.one(ctx, updateDetailsSQL, id, row.Name, row.CNICCipher, row.CNICLookup,
		row.VehicleType, nullEmpty(row.VehicleReg), nullEmpty(row.LicenseNumber))
}

func (r *Repository) UpdateDocuments(ctx context.Context, id uuid.UUID, in DocumentsInput) (Profile, error) {
	return r.one(ctx, updateDocumentsSQL, id, in.CNICFrontObjectKey, in.CNICBackObjectKey,
		in.LicenseObjectKey, in.SelfieObjectKey)
}

func (r *Repository) Submit(ctx context.Context, id uuid.UUID) (Profile, error) {
	return r.one(ctx, submitSQL, id)
}

func (r *Repository) BookOrientation(ctx context.Context, id uuid.UUID, slot *string) (Profile, error) {
	return r.one(ctx, bookOrientationSQL, id, slot)
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

type detailsRow struct {
	Name          string
	CNICCipher    string
	CNICLookup    string
	VehicleType   string
	VehicleReg    string
	LicenseNumber string
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
		&profile.ID, &profile.Name, &profile.VerificationStatus, &profile.ZoneID, &profile.VehicleReg,
		&profile.VehicleType, &profile.LicenseNumber, &profile.IsOnline, &profile.Rating, &profile.RatingCount,
		&profile.CNICObjectKey, &profile.CNICFrontObjectKey, &profile.CNICBackObjectKey,
		&profile.SelfieObjectKey, &profile.LicenseObjectKey,
		&profile.OrientationStatus, &profile.OrientationSlot, &profile.ApplicationReceived,
		&profile.OnboardingStep, &profile.CreatedAt,
	)
	if err != nil {
		return Profile{}, err
	}
	return profile, nil
}

func nullEmpty(raw string) *string {
	if raw == "" {
		return nil
	}
	return &raw
}

func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
