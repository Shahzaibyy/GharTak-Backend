package admin

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
	"github.com/yourusername/ghartak-backend/internal/platform/money"
)

const listZonesSQL = `
SELECT
    id,
    city_name,
    slug,
    base_delivery_fee::text,
    per_km_rate::text,
    surge_multiplier::text,
    service_radius_km::text,
    is_active,
    created_at,
    updated_at
FROM zones
WHERE ($1::boolean = false OR is_active = true)
ORDER BY city_name
LIMIT 20`

const ensureAdminSQL = `
INSERT INTO admins (phone_ciphertext, phone_lookup, name, status)
VALUES ($1, $2, $3, 'active')
ON CONFLICT (phone_lookup) DO NOTHING`

const getZoneSQL = `
SELECT
    id,
    city_name,
    slug,
    base_delivery_fee::text,
    per_km_rate::text,
    surge_multiplier::text,
    service_radius_km::text,
    is_active,
    created_at,
    updated_at
FROM zones
WHERE id = $1`

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) List(ctx context.Context, activeOnly bool) ([]Zone, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	rows, err := r.pool.Query(ctx, listZonesSQL, activeOnly)
	if err != nil {
		return nil, fmt.Errorf("admin: list zones: %w", err)
	}
	defer rows.Close()
	return scanZones(rows)
}

func (r *Repository) EnsureAdmin(ctx context.Context, ciphertext, lookup, name string) error {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if _, err := r.pool.Exec(ctx, ensureAdminSQL, ciphertext, lookup, name); err != nil {
		return fmt.Errorf("admin: seed admin: %w", err)
	}
	return nil
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (Zone, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	zone, err := scanZone(r.pool.QueryRow(ctx, getZoneSQL, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Zone{}, apperror.ErrNotFound
	}
	if err != nil {
		return Zone{}, fmt.Errorf("admin: get zone: %w", err)
	}
	return zone, nil
}

func scanZones(rows pgx.Rows) ([]Zone, error) {
	zones := make([]Zone, 0)
	for rows.Next() {
		zone, err := scanZone(rows)
		if err != nil {
			return nil, fmt.Errorf("admin: scan zone: %w", err)
		}
		zones = append(zones, zone)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("admin: list zones: %w", err)
	}
	return zones, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanZone(row rowScanner) (Zone, error) {
	var zone Zone
	var baseFee, perKm string
	err := row.Scan(
		&zone.ID,
		&zone.CityName,
		&zone.Slug,
		&baseFee,
		&perKm,
		&zone.SurgeMultiplier,
		&zone.ServiceRadiusKm,
		&zone.IsActive,
		&zone.CreatedAt,
		&zone.UpdatedAt,
	)
	if err != nil {
		return Zone{}, err
	}
	return parseZoneMoney(zone, baseFee, perKm)
}

func parseZoneMoney(zone Zone, baseFee, perKm string) (Zone, error) {
	fee, err := money.Parse(baseFee)
	if err != nil {
		return Zone{}, fmt.Errorf("admin: parse base fee: %w", err)
	}
	rate, err := money.Parse(perKm)
	if err != nil {
		return Zone{}, fmt.Errorf("admin: parse per km rate: %w", err)
	}
	zone.BaseDeliveryFee = fee
	zone.PerKmRate = rate
	return zone, nil
}
