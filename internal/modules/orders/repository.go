package orders

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
	"github.com/yourusername/ghartak-backend/internal/platform/money"
)

// Payments inserted here are held rows only. Ledger journals belong to the payments module.

const orderColumns = `
id, type, customer_id, merchant_id, zone_id, status,
pickup_lat, pickup_lng, drop_lat, drop_lng,
COALESCE(pickup_address, ''), COALESCE(drop_address, ''),
description, effort_tier, distance_km::text, item_total::text, delivery_fee::text,
commission_amount::text, rider_earning::text, surge_multiplier::text,
payment_method, COALESCE(client_request_id, ''), created_at`

const insertOrderSQL = `
INSERT INTO orders (
    type, customer_id, merchant_id, zone_id,
    pickup_lat, pickup_lng, drop_lat, drop_lng,
    pickup_address, drop_address,
    status, description, effort_tier,
    distance_km, item_total, delivery_fee, commission_amount, rider_earning, surge_multiplier,
    payment_method, client_request_id
) VALUES (
    $1, $2, $3, $4,
    $5, $6, $7, $8,
    $9, $10,
    $11, $12, $13,
    $14, $15, $16, $17, $18, $19,
    $20, $21
)
RETURNING id, created_at`

const insertItemSQL = `
INSERT INTO order_items (order_id, catalog_item_id, item_name, quantity, price_at_order)
VALUES ($1, $2, $3, $4, $5)`

const insertEventSQL = `
INSERT INTO order_events (order_id, from_status, to_status, actor_role, actor_id)
VALUES ($1, NULL, $2, $3, $4)`

const insertPaymentSQL = `
INSERT INTO payments (order_id, amount, method, status)
VALUES ($1, $2, $3, 'held')`

const findRequestSQL = `
SELECT ` + orderColumns + `
FROM orders
WHERE customer_id = $1 AND client_request_id = $2`

const getOrderSQL = `
SELECT ` + orderColumns + `
FROM orders
WHERE id = $1 AND customer_id = $2`

const listItemsSQL = `
SELECT catalog_item_id, item_name, quantity, price_at_order::text
FROM order_items
WHERE order_id = $1
ORDER BY id
LIMIT 50`

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) FindByClientRequest(ctx context.Context, customerID uuid.UUID, requestID string) (Order, error) {
	return r.load(ctx, findRequestSQL, customerID, requestID)
}

func (r *Repository) Get(ctx context.Context, customerID, orderID uuid.UUID) (Order, error) {
	return r.load(ctx, getOrderSQL, orderID, customerID)
}

func (r *Repository) Insert(ctx context.Context, row draft) (Order, bool, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Order{}, false, fmt.Errorf("orders: begin: %w", err)
	}
	order, err := writeOrder(ctx, tx, row)
	if err != nil {
		return r.afterWrite(ctx, tx, err, row)
	}
	if err := tx.Commit(ctx); err != nil {
		return Order{}, false, fmt.Errorf("orders: commit: %w", err)
	}
	return order, true, nil
}

func (r *Repository) afterWrite(ctx context.Context, tx pgx.Tx, err error, row draft) (Order, bool, error) {
	_ = tx.Rollback(ctx)
	if !isUnique(err) {
		return Order{}, false, err
	}
	return r.replay(ctx, err, row)
}

func (r *Repository) replay(ctx context.Context, cause error, row draft) (Order, bool, error) {
	existing, err := r.load(ctx, findRequestSQL, row.CustomerID, row.Input.ClientRequestID)
	if errors.Is(err, apperror.ErrNotFound) {
		return Order{}, false, cause
	}
	if err != nil {
		return Order{}, false, err
	}
	return existing, false, nil
}

func (r *Repository) load(ctx context.Context, query string, args ...any) (Order, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	order, err := scanOrder(r.pool.QueryRow(ctx, query, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, apperror.ErrNotFound
	}
	if err != nil {
		return Order{}, fmt.Errorf("orders: load: %w", err)
	}
	return r.withItems(ctx, order)
}

func (r *Repository) withItems(ctx context.Context, order Order) (Order, error) {
	rows, err := r.pool.Query(ctx, listItemsSQL, order.ID)
	if err != nil {
		return Order{}, fmt.Errorf("orders: items: %w", err)
	}
	defer rows.Close()
	items, err := scanItems(rows)
	if err != nil {
		return Order{}, err
	}
	order.Items = items
	return order, nil
}

func writeOrder(ctx context.Context, tx pgx.Tx, row draft) (Order, error) {
	order, err := insertOrder(ctx, tx, row)
	if err != nil {
		return Order{}, err
	}
	if err := insertChildren(ctx, tx, order, row); err != nil {
		return Order{}, err
	}
	return order, nil
}

func insertChildren(ctx context.Context, tx pgx.Tx, order Order, row draft) error {
	if err := insertItems(ctx, tx, order.ID, row.Items); err != nil {
		return err
	}
	if err := insertEvent(ctx, tx, order); err != nil {
		return err
	}
	return insertPayment(ctx, tx, order, row.Price.Total.String())
}

func insertOrder(ctx context.Context, tx pgx.Tx, row draft) (Order, error) {
	order := orderFrom(row)
	err := tx.QueryRow(ctx, insertOrderSQL, insertArgs(row)...).Scan(&order.ID, &order.CreatedAt)
	if err != nil {
		return Order{}, fmt.Errorf("orders: insert: %w", err)
	}
	return order, nil
}

func insertItems(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, items []OrderItem) error {
	for _, item := range items {
		if err := insertItem(ctx, tx, orderID, item); err != nil {
			return err
		}
	}
	return nil
}

func insertItem(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, item OrderItem) error {
	_, err := tx.Exec(ctx, insertItemSQL, orderID, item.CatalogItemID, item.Name, item.Quantity, item.Price.String())
	if err != nil {
		return fmt.Errorf("orders: insert item: %w", err)
	}
	return nil
}

func insertEvent(ctx context.Context, tx pgx.Tx, order Order) error {
	_, err := tx.Exec(ctx, insertEventSQL, order.ID, string(StatusPlaced), string(domain.RoleCustomer), order.CustomerID)
	if err != nil {
		return fmt.Errorf("orders: insert event: %w", err)
	}
	return nil
}

func insertPayment(ctx context.Context, tx pgx.Tx, order Order, amount string) error {
	_, err := tx.Exec(ctx, insertPaymentSQL, order.ID, amount, order.PaymentMethod)
	if err != nil {
		return fmt.Errorf("orders: insert payment: %w", err)
	}
	return nil
}

func orderFrom(row draft) Order {
	order := Order{
		Type: row.Input.Type, CustomerID: row.CustomerID, MerchantID: row.Input.MerchantID,
		ZoneID: row.Input.ZoneID, Status: StatusPlaced,
		PickupLat: row.PickupLat, PickupLng: row.PickupLng, DropLat: row.Input.DropLat, DropLng: row.Input.DropLng,
		PickupAddress: row.PickupAddress, DropAddress: row.Input.DropAddress,
		Description: textPtr(row.Input.Description), EffortTier: effortPtr(row.Input.Effort),
		PaymentMethod: row.Input.PaymentMethod, ClientRequestID: row.Input.ClientRequestID,
		Items: row.Items, Priced: row.Price,
	}
	return order
}

func insertArgs(row draft) []any {
	price := row.Price
	return []any{
		string(row.Input.Type), row.CustomerID, row.Input.MerchantID, row.Input.ZoneID,
		row.PickupLat, row.PickupLng, row.Input.DropLat, row.Input.DropLng,
		row.PickupAddress, row.Input.DropAddress,
		string(StatusPlaced), textArg(row.Input.Description), effortArg(row.Input.Effort),
		price.DistanceKm, price.ItemTotal.String(), price.DeliveryFee.String(), price.Commission.String(), price.RiderEarning.String(), price.SurgeMultiplier,
		row.Input.PaymentMethod, row.Input.ClientRequestID,
	}
}

func scanOrder(row pgx.Row) (Order, error) {
	var order Order
	var distance, itemTotal, fee, commission, rider string
	err := row.Scan(
		&order.ID, &order.Type, &order.CustomerID, &order.MerchantID, &order.ZoneID, &order.Status,
		&order.PickupLat, &order.PickupLng, &order.DropLat, &order.DropLng,
		&order.PickupAddress, &order.DropAddress,
		&order.Description, &order.EffortTier, &distance, &itemTotal, &fee,
		&commission, &rider, &order.SurgeMultiplier,
		&order.PaymentMethod, &order.ClientRequestID, &order.CreatedAt,
	)
	if err != nil {
		return Order{}, err
	}
	return fillMoney(order, distance, itemTotal, fee, commission, rider)
}

func fillMoney(order Order, distance, itemTotal, fee, commission, rider string) (Order, error) {
	parsed, err := parseAmounts(itemTotal, fee, commission, rider)
	if err != nil {
		return Order{}, err
	}
	order.DistanceKm = distance
	order.ItemTotal = parsed[0]
	order.DeliveryFee = parsed[1]
	order.Commission = parsed[2]
	order.RiderEarning = parsed[3]
	order.Total, err = order.ItemTotal.Add(order.DeliveryFee)
	if err != nil {
		return Order{}, err
	}
	return order, nil
}

func parseAmounts(itemTotal, fee, commission, rider string) ([]money.Money, error) {
	raw := []string{itemTotal, fee, commission, rider}
	out := make([]money.Money, len(raw))
	for i, value := range raw {
		parsed, err := money.Parse(value)
		if err != nil {
			return nil, fmt.Errorf("orders: parse money: %w", err)
		}
		out[i] = parsed
	}
	return out, nil
}

func scanItems(rows pgx.Rows) ([]OrderItem, error) {
	items := make([]OrderItem, 0)
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, fmt.Errorf("orders: items: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("orders: items: %w", err)
	}
	return items, nil
}

func scanItem(row pgx.Row) (OrderItem, error) {
	var item OrderItem
	var price string
	if err := row.Scan(&item.CatalogItemID, &item.Name, &item.Quantity, &price); err != nil {
		return OrderItem{}, err
	}
	parsed, err := money.Parse(price)
	if err != nil {
		return OrderItem{}, fmt.Errorf("orders: parse item price: %w", err)
	}
	item.Price = parsed
	return item, nil
}

func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func textArg(raw string) any {
	if raw == "" {
		return nil
	}
	return raw
}

func textPtr(raw string) *string {
	if raw == "" {
		return nil
	}
	return &raw
}

func effortArg(tier *EffortTier) any {
	if tier == nil {
		return nil
	}
	return string(*tier)
}

func effortPtr(tier *EffortTier) *string {
	if tier == nil {
		return nil
	}
	value := string(*tier)
	return &value
}
