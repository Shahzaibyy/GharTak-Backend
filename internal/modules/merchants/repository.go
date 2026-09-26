package merchants

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
	"github.com/yourusername/ghartak-backend/internal/platform/money"
)

const insertMerchantSQL = `
INSERT INTO merchants (
    phone_ciphertext, phone_lookup, owner_name, name, category, zone_id,
    lat, lng, address_text, commission_rate, verification_status
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'pending')
RETURNING id, name, category, zone_id, lat, lng, address_text, photo_key, commission_rate::text, verification_status`

const listMerchantsSQL = `
SELECT id, name, category, zone_id, lat, lng, address_text, photo_key, commission_rate::text, verification_status
FROM merchants
WHERE zone_id = $1 AND verification_status = 'approved'
ORDER BY name
LIMIT 20`

const listCatalogSQL = `
SELECT id, merchant_id, name, description, price::text, is_available, photo_key
FROM catalog_items
WHERE merchant_id = $1 AND is_available = true
ORDER BY name
LIMIT 50`

const insertCatalogSQL = `
INSERT INTO catalog_items (merchant_id, name, description, price, is_available, photo_key)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, merchant_id, name, description, price::text, is_available, photo_key`

const updateCatalogSQL = `
UPDATE catalog_items
SET name = $3, description = $4, price = $5, is_available = $6, photo_key = $7, updated_at = now()
WHERE id = $1 AND merchant_id = $2
RETURNING id, merchant_id, name, description, price::text, is_available, photo_key`

const deleteCatalogSQL = `
DELETE FROM catalog_items
WHERE id = $1 AND merchant_id = $2`

const pinSQL = `
SELECT id, lat, lng, address_text, commission_rate::text
FROM merchants
WHERE id = $1 AND zone_id = $2 AND verification_status = 'approved'`

const pricesSQL = `
SELECT id, name, price::text
FROM catalog_items
WHERE merchant_id = $1 AND is_available = true AND id = ANY($2)`

const ensureDemoSQL = `
INSERT INTO merchants (
    id, phone_ciphertext, phone_lookup, owner_name, name, category, zone_id,
    lat, lng, address_text, commission_rate, verification_status
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'approved')
ON CONFLICT (phone_lookup) DO NOTHING`

const merchantByLookupSQL = `
SELECT id FROM merchants WHERE phone_lookup = $1`

const setVerificationSQL = `
UPDATE merchants
SET verification_status = $2, updated_at = now()
WHERE id = $1 AND verification_status = 'pending'
RETURNING id, name, category, zone_id, lat, lng, address_text, photo_key, commission_rate::text, verification_status`

const catalogCountSQL = `
SELECT COUNT(*) FROM catalog_items WHERE merchant_id = $1`

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Insert(ctx context.Context, row merchantRow) (Merchant, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	merchant, err := scanMerchant(r.pool.QueryRow(ctx, insertMerchantSQL,
		row.Ciphertext, row.Lookup, row.OwnerName, row.Name, row.Category, row.ZoneID,
		row.Lat, row.Lng, row.AddressText, row.Commission,
	))
	if isUnique(err) {
		return Merchant{}, apperror.ErrConflict
	}
	if err != nil {
		return Merchant{}, fmt.Errorf("merchants: insert: %w", err)
	}
	return merchant, nil
}

func (r *Repository) ListApproved(ctx context.Context, zoneID uuid.UUID) ([]Merchant, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	rows, err := r.pool.Query(ctx, listMerchantsSQL, zoneID)
	if err != nil {
		return nil, fmt.Errorf("merchants: list: %w", err)
	}
	defer rows.Close()
	return scanMerchants(rows)
}

func (r *Repository) ListAvailable(ctx context.Context, merchantID uuid.UUID) ([]CatalogItem, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	rows, err := r.pool.Query(ctx, listCatalogSQL, merchantID)
	if err != nil {
		return nil, fmt.Errorf("merchants: catalog: %w", err)
	}
	defer rows.Close()
	return scanCatalog(rows)
}

func (r *Repository) InsertItem(ctx context.Context, merchantID uuid.UUID, item catalogRow) (CatalogItem, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return scanItem(r.pool.QueryRow(ctx, insertCatalogSQL,
		merchantID, item.Name, item.Description, item.Price, item.IsAvailable, item.PhotoKey,
	))
}

func (r *Repository) UpdateItem(ctx context.Context, merchantID, itemID uuid.UUID, item catalogRow) (CatalogItem, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	found, err := scanItem(r.pool.QueryRow(ctx, updateCatalogSQL,
		itemID, merchantID, item.Name, item.Description, item.Price, item.IsAvailable, item.PhotoKey,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return CatalogItem{}, apperror.ErrNotFound
	}
	return found, err
}

func (r *Repository) DeleteItem(ctx context.Context, merchantID, itemID uuid.UUID) error {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tag, err := r.pool.Exec(ctx, deleteCatalogSQL, itemID, merchantID)
	if err != nil {
		return fmt.Errorf("merchants: delete catalog: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

func (r *Repository) Pin(ctx context.Context, merchantID, zoneID uuid.UUID) (Pin, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var pin Pin
	err := r.pool.QueryRow(ctx, pinSQL, merchantID, zoneID).Scan(&pin.ID, &pin.Lat, &pin.Lng, &pin.AddressText, &pin.CommissionRate)
	if errors.Is(err, pgx.ErrNoRows) {
		return Pin{}, apperror.ErrNotFound
	}
	if err != nil {
		return Pin{}, fmt.Errorf("merchants: pin: %w", err)
	}
	return pin, nil
}

func (r *Repository) EnsureDemo(ctx context.Context, row demoRow) (uuid.UUID, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if _, err := r.pool.Exec(ctx, ensureDemoSQL, row.args()...); err != nil {
		return uuid.UUID{}, fmt.Errorf("merchants: seed: %w", err)
	}
	return r.lookupMerchant(ctx, row.Lookup)
}

func (r *Repository) lookupMerchant(ctx context.Context, lookup string) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, merchantByLookupSQL, lookup).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.UUID{}, apperror.ErrNotFound
	}
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("merchants: seed lookup: %w", err)
	}
	return id, nil
}

func (r *Repository) CatalogCount(ctx context.Context, merchantID uuid.UUID) (int, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var count int
	if err := r.pool.QueryRow(ctx, catalogCountSQL, merchantID).Scan(&count); err != nil {
		return 0, fmt.Errorf("merchants: catalog count: %w", err)
	}
	return count, nil
}

func (r *Repository) SetVerification(ctx context.Context, id uuid.UUID, status string) (Merchant, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	merchant, err := scanMerchant(r.pool.QueryRow(ctx, setVerificationSQL, id, status))
	if errors.Is(err, pgx.ErrNoRows) {
		return Merchant{}, apperror.ErrNotFound
	}
	if err != nil {
		return Merchant{}, fmt.Errorf("merchants: set verification: %w", err)
	}
	return merchant, nil
}

func (r *Repository) Prices(ctx context.Context, merchantID uuid.UUID, ids []uuid.UUID) ([]Price, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	rows, err := r.pool.Query(ctx, pricesSQL, merchantID, ids)
	if err != nil {
		return nil, fmt.Errorf("merchants: prices: %w", err)
	}
	defer rows.Close()
	return scanPrices(rows)
}

type merchantRow struct {
	Ciphertext  string
	Lookup      string
	OwnerName   string
	Name        string
	Category    Category
	ZoneID      uuid.UUID
	Lat         float64
	Lng         float64
	AddressText string
	Commission  string
}

type demoRow struct {
	ID          uuid.UUID
	Ciphertext  string
	Lookup      string
	OwnerName   string
	Name        string
	Category    Category
	ZoneID      uuid.UUID
	Lat         float64
	Lng         float64
	AddressText string
	Commission  string
}

func (row demoRow) args() []any {
	return []any{
		row.ID, row.Ciphertext, row.Lookup, row.OwnerName, row.Name, row.Category,
		row.ZoneID, row.Lat, row.Lng, row.AddressText, row.Commission,
	}
}

type catalogRow struct {
	Name        string
	Description any
	Price       string
	IsAvailable bool
	PhotoKey    any
}

func scanMerchants(rows pgx.Rows) ([]Merchant, error) {
	out := make([]Merchant, 0)
	for rows.Next() {
		merchant, err := scanMerchant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, merchant)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("merchants: list: %w", err)
	}
	return out, nil
}

func scanMerchant(row pgx.Row) (Merchant, error) {
	var merchant Merchant
	err := row.Scan(
		&merchant.ID, &merchant.Name, &merchant.Category, &merchant.ZoneID, &merchant.Lat, &merchant.Lng,
		&merchant.AddressText, &merchant.PhotoKey, &merchant.CommissionRate, &merchant.VerificationStatus,
	)
	if err != nil {
		return Merchant{}, err
	}
	return merchant, nil
}

func scanCatalog(rows pgx.Rows) ([]CatalogItem, error) {
	out := make([]CatalogItem, 0)
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, fmt.Errorf("merchants: catalog: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("merchants: catalog: %w", err)
	}
	return out, nil
}

func scanItem(row pgx.Row) (CatalogItem, error) {
	var item CatalogItem
	var price string
	err := row.Scan(&item.ID, &item.MerchantID, &item.Name, &item.Description, &price, &item.IsAvailable, &item.PhotoKey)
	if err != nil {
		return CatalogItem{}, err
	}
	return parseItemPrice(item, price)
}

func parseItemPrice(item CatalogItem, raw string) (CatalogItem, error) {
	price, err := money.Parse(raw)
	if err != nil {
		return CatalogItem{}, fmt.Errorf("merchants: parse price: %w", err)
	}
	item.Price = price
	return item, nil
}

func scanPrices(rows pgx.Rows) ([]Price, error) {
	out := make([]Price, 0)
	for rows.Next() {
		price, err := scanPrice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, price)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("merchants: prices: %w", err)
	}
	return out, nil
}

func scanPrice(row pgx.Row) (Price, error) {
	var price Price
	var raw string
	if err := row.Scan(&price.ID, &price.Name, &raw); err != nil {
		return Price{}, err
	}
	parsed, err := money.Parse(raw)
	if err != nil {
		return Price{}, fmt.Errorf("merchants: parse price: %w", err)
	}
	price.Price = parsed
	return price, nil
}

func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
