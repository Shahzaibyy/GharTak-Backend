package dispatch

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yourusername/ghartak-backend/internal/modules/orders"
	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/database"
)

type View struct {
	ID        uuid.UUID
	Type      orders.Type
	Status    orders.Status
	ZoneID    uuid.UUID
	PickupLat float64
	PickupLng float64
	RadiusKm  int
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) View(ctx context.Context, orderID uuid.UUID) (View, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var view View
	var radius string
	err := r.pool.QueryRow(ctx, `
SELECT o.id, o.type, o.status, o.zone_id, o.pickup_lat, o.pickup_lng, z.service_radius_km::text
FROM orders o
JOIN zones z ON z.id = o.zone_id
WHERE o.id = $1`, orderID).Scan(&view.ID, &view.Type, &view.Status, &view.ZoneID, &view.PickupLat, &view.PickupLng, &radius)
	if errors.Is(err, pgx.ErrNoRows) {
		return View{}, apperror.ErrNotFound
	}
	if err != nil {
		return View{}, fmt.Errorf("dispatch: view: %w", err)
	}
	view.RadiusKm, err = radiusKm(radius)
	if err != nil {
		return View{}, err
	}
	return view, nil
}

func (r *Repository) Offered(ctx context.Context, orderID uuid.UUID) (map[uuid.UUID]struct{}, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	rows, err := r.pool.Query(ctx, `SELECT rider_id FROM order_offers WHERE order_id = $1 LIMIT 50`, orderID)
	if err != nil {
		return nil, fmt.Errorf("dispatch: offers: %w", err)
	}
	defer rows.Close()
	return scanIDs(rows)
}

func (r *Repository) Expire(ctx context.Context, orderID uuid.UUID) error {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := r.pool.Exec(ctx, `
UPDATE order_offers SET response = 'expired', responded_at = now()
WHERE order_id = $1 AND response IS NULL AND offered_at < now() - interval '30 seconds'`, orderID)
	if err != nil {
		return fmt.Errorf("dispatch: expire: %w", err)
	}
	return nil
}

func (r *Repository) SaveOffer(ctx context.Context, view View, riderID uuid.UUID) error {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("dispatch: begin: %w", err)
	}
	if err := saveOfferTx(ctx, tx, view, riderID); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("dispatch: commit: %w", err)
	}
	return nil
}

func saveOfferTx(ctx context.Context, tx pgx.Tx, view View, riderID uuid.UUID) error {
	if err := markOffered(ctx, tx, view); err != nil {
		return err
	}
	if err := insertOffer(ctx, tx, view.ID, riderID); err != nil {
		return err
	}
	return offerEvent(ctx, tx, view)
}

func markOffered(ctx context.Context, tx pgx.Tx, view View) error {
	tag, err := tx.Exec(ctx, `UPDATE orders SET status = 'rider_offered' WHERE id = $1 AND status = $2`, view.ID, string(view.Status))
	if err != nil {
		return fmt.Errorf("dispatch: status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperror.Conflict("order status changed")
	}
	return nil
}

func insertOffer(ctx context.Context, tx pgx.Tx, orderID, riderID uuid.UUID) error {
	_, err := tx.Exec(ctx, `INSERT INTO order_offers (order_id, rider_id) VALUES ($1, $2)`, orderID, riderID)
	if isUnique(err) {
		return apperror.Conflict("rider was already offered this order")
	}
	if err != nil {
		return fmt.Errorf("dispatch: offer: %w", err)
	}
	return nil
}

func offerEvent(ctx context.Context, tx pgx.Tx, view View) error {
	if view.Status == orders.StatusRiderOffered {
		return nil
	}
	_, err := tx.Exec(ctx, `
INSERT INTO order_events (order_id, from_status, to_status, actor_role, actor_id)
VALUES ($1, $2, 'rider_offered', 'system', NULL)`, view.ID, string(view.Status))
	if err != nil {
		return fmt.Errorf("dispatch: event: %w", err)
	}
	return nil
}

func scanIDs(rows pgx.Rows) (map[uuid.UUID]struct{}, error) {
	out := make(map[uuid.UUID]struct{})
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("dispatch: offers: %w", err)
		}
		out[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("dispatch: offers: %w", err)
	}
	return out, nil
}

// OnlineInZone returns any approved online rider in the zone, skipping prior offers.
// Used when Redis geo has no live hit (presence TTL expired or GEOSEARCH unavailable).
func (r *Repository) OnlineInZone(ctx context.Context, zoneID uuid.UUID, skip map[uuid.UUID]struct{}) (uuid.UUID, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	rows, err := r.pool.Query(ctx, `
SELECT id FROM riders
WHERE zone_id = $1 AND is_online = true AND verification_status = 'approved'
ORDER BY updated_at DESC
LIMIT 20`, zoneID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("dispatch: online riders: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return uuid.Nil, fmt.Errorf("dispatch: online riders: %w", err)
		}
		if skipped(skip, id) {
			continue
		}
		return id, nil
	}
	if err := rows.Err(); err != nil {
		return uuid.Nil, fmt.Errorf("dispatch: online riders: %w", err)
	}
	return uuid.Nil, nil
}

func radiusKm(raw string) (int, error) {
	km, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(km) || km <= 0 {
		return 0, fmt.Errorf("dispatch: radius: invalid service_radius_km %q", raw)
	}
	return int(math.Round(km)), nil
}

func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
