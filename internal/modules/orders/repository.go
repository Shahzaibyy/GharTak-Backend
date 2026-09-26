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

	"github.com/yourusername/ghartak-backend/internal/modules/payments"
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
payment_method, COALESCE(client_request_id, ''), created_at, rider_id, COALESCE(delivery_otp_hash, '')`

const insertOrderSQL = `
INSERT INTO orders (
    type, customer_id, merchant_id, zone_id,
    pickup_lat, pickup_lng, drop_lat, drop_lng,
    pickup_address, drop_address,
    status, description, effort_tier,
    distance_km, item_total, delivery_fee, commission_amount, rider_earning, surge_multiplier,
    payment_method, client_request_id, delivery_otp_hash
) VALUES (
    $1, $2, $3, $4,
    $5, $6, $7, $8,
    $9, $10,
    $11, $12, $13,
    $14, $15, $16, $17, $18, $19,
    $20, $21, $22
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
VALUES ($1, $2, $3, 'held')
RETURNING id`

const getByIDSQL = `
SELECT ` + orderColumns + `
FROM orders
WHERE id = $1`

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

type books interface {
	Hold(ctx context.Context, tx pgx.Tx, in payments.HoldInput) error
	Release(ctx context.Context, tx pgx.Tx, in payments.ReleaseInput) error
	Refund(ctx context.Context, tx pgx.Tx, in payments.RefundInput) error
}

type Repository struct {
	pool  *pgxpool.Pool
	books books
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) WithBooks(books books) *Repository {
	r.books = books
	return r
}

func (r *Repository) FindByClientRequest(ctx context.Context, customerID uuid.UUID, requestID string) (Order, error) {
	return r.load(ctx, findRequestSQL, customerID, requestID)
}

func (r *Repository) Get(ctx context.Context, customerID, orderID uuid.UUID) (Order, error) {
	return r.load(ctx, getOrderSQL, orderID, customerID)
}

func (r *Repository) GetByID(ctx context.Context, orderID uuid.UUID) (Order, error) {
	return r.load(ctx, getByIDSQL, orderID)
}

func (r *Repository) Insert(ctx context.Context, row draft) (Order, bool, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Order{}, false, fmt.Errorf("orders: begin: %w", err)
	}
	order, err := r.writeOrder(ctx, tx, row)
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

func (r *Repository) writeOrder(ctx context.Context, tx pgx.Tx, row draft) (Order, error) {
	order, err := insertOrder(ctx, tx, row)
	if err != nil {
		return Order{}, err
	}
	if err := r.insertChildren(ctx, tx, order, row); err != nil {
		return Order{}, err
	}
	return order, nil
}

func (r *Repository) insertChildren(ctx context.Context, tx pgx.Tx, order Order, row draft) error {
	if err := insertItems(ctx, tx, order.ID, row.Items); err != nil {
		return err
	}
	if err := insertEvent(ctx, tx, order); err != nil {
		return err
	}
	paymentID, err := insertPayment(ctx, tx, order, row.Price.Total.String())
	if err != nil {
		return err
	}
	return r.postHold(ctx, tx, order, paymentID)
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

func insertPayment(ctx context.Context, tx pgx.Tx, order Order, amount string) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, insertPaymentSQL, order.ID, amount, order.PaymentMethod).Scan(&id)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("orders: insert payment: %w", err)
	}
	return id, nil
}

func (r *Repository) postHold(ctx context.Context, tx pgx.Tx, order Order, paymentID uuid.UUID) error {
	if r.books == nil {
		return nil
	}
	return r.books.Hold(ctx, tx, payments.HoldInput{
		OrderID: order.ID, PaymentID: paymentID, CustomerID: order.CustomerID,
		Method: payments.Method(order.PaymentMethod), Total: order.Total,
	})
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
		row.Input.PaymentMethod, row.Input.ClientRequestID, textArg(row.DeliveryHash),
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
		&order.RiderID, &order.DeliveryHash,
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

type advance struct {
	OrderID        uuid.UUID
	To             Status
	ActorRole      string
	ActorID        uuid.UUID
	ExpectMerchant *uuid.UUID
	ExpectRider    *uuid.UUID
}

type deliverInput struct {
	RiderID uuid.UUID
	OrderID uuid.UUID
	OTP     string
	Proof   string
	HashKey []byte
}

type paymentRow struct {
	ID     uuid.UUID
	Status string
}

const partySQL = `
SELECT id, type, status, customer_id, rider_id, merchant_id, zone_id, pickup_lat, pickup_lng
FROM orders WHERE id = $1`

const lockOrderSQL = `
SELECT ` + orderColumns + `
FROM orders WHERE id = $1 FOR UPDATE`

const lockPaymentSQL = `
SELECT id, status FROM payments WHERE order_id = $1 FOR UPDATE`

func (r *Repository) Party(ctx context.Context, orderID uuid.UUID) (Party, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var party Party
	err := r.pool.QueryRow(ctx, partySQL, orderID).Scan(
		&party.OrderID, &party.Type, &party.Status, &party.CustomerID, &party.RiderID, &party.MerchantID,
		&party.ZoneID, &party.PickupLat, &party.PickupLng,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Party{}, apperror.ErrNotFound
	}
	if err != nil {
		return Party{}, fmt.Errorf("orders: party: %w", err)
	}
	return party, nil
}

func (r *Repository) ListByMerchant(ctx context.Context, merchantID uuid.UUID, status Status) ([]Order, error) {
	return r.list(ctx, `
SELECT `+orderColumns+`
FROM orders
WHERE merchant_id = $1 AND status = $2
ORDER BY created_at DESC
LIMIT 20`, merchantID, string(status))
}

func (r *Repository) ListOffers(ctx context.Context, riderID uuid.UUID) ([]Order, error) {
	return r.list(ctx, `
SELECT `+orderColumns+`
FROM orders
WHERE id IN (
    SELECT order_id FROM order_offers
    WHERE rider_id = $1 AND response IS NULL
    ORDER BY offered_at DESC
    LIMIT 20
)
LIMIT 20`, riderID)
}

func (r *Repository) ListTasks(ctx context.Context, riderID uuid.UUID) ([]Order, error) {
	return r.list(ctx, `
SELECT `+orderColumns+`
FROM orders
WHERE rider_id = $1 AND status = ANY($2)
ORDER BY created_at DESC
LIMIT 20`, riderID, []string{string(StatusAccepted), string(StatusPickedUp), string(StatusOnTheWay)})
}

func (r *Repository) list(ctx context.Context, query string, args ...any) ([]Order, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("orders: list: %w", err)
	}
	defer rows.Close()
	return scanOrders(rows)
}

func scanOrders(rows pgx.Rows) ([]Order, error) {
	out := make([]Order, 0)
	for rows.Next() {
		order, err := scanOrder(rows)
		if err != nil {
			return nil, fmt.Errorf("orders: list: %w", err)
		}
		out = append(out, order)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("orders: list: %w", err)
	}
	return out, nil
}

func (r *Repository) Advance(ctx context.Context, in advance) (Order, error) {
	return r.transact(ctx, func(tx pgx.Tx) (Order, error) {
		return r.advanceTx(ctx, tx, in)
	})
}

func (r *Repository) AcceptOffer(ctx context.Context, riderID, orderID uuid.UUID) (Order, error) {
	return r.transact(ctx, func(tx pgx.Tx) (Order, error) {
		return acceptTx(ctx, tx, riderID, orderID)
	})
}

func (r *Repository) RejectOffer(ctx context.Context, riderID, orderID uuid.UUID) (Order, error) {
	return r.transact(ctx, func(tx pgx.Tx) (Order, error) {
		return rejectTx(ctx, tx, riderID, orderID)
	})
}

func (r *Repository) Deliver(ctx context.Context, in deliverInput) (Order, error) {
	return r.transact(ctx, func(tx pgx.Tx) (Order, error) {
		return r.deliverTx(ctx, tx, in)
	})
}

func (r *Repository) Cancel(ctx context.Context, customerID, orderID uuid.UUID, reason string) (Order, error) {
	return r.transact(ctx, func(tx pgx.Tx) (Order, error) {
		return r.cancelTx(ctx, tx, customerID, orderID, reason)
	})
}

func (r *Repository) transact(ctx context.Context, fn func(pgx.Tx) (Order, error)) (Order, error) {
	ctx, cancel := database.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Order{}, fmt.Errorf("orders: begin: %w", err)
	}
	order, err := fn(tx)
	if err != nil {
		_ = tx.Rollback(ctx)
		return Order{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Order{}, fmt.Errorf("orders: commit: %w", err)
	}
	return order, nil
}

func (r *Repository) advanceTx(ctx context.Context, tx pgx.Tx, in advance) (Order, error) {
	order, err := lockOrder(ctx, tx, in.OrderID)
	if err != nil {
		return Order{}, err
	}
	if err := authorizeAdvance(order, in); err != nil {
		return Order{}, err
	}
	if err := applyStatus(ctx, tx, order, in.To, in.ActorRole, in.ActorID, ""); err != nil {
		return Order{}, err
	}
	order.Status = in.To
	return order, nil
}

func (r *Repository) deliverTx(ctx context.Context, tx pgx.Tx, in deliverInput) (Order, error) {
	order, err := lockOrder(ctx, tx, in.OrderID)
	if err != nil {
		return Order{}, err
	}
	if err := allowDeliver(order, in); err != nil {
		return Order{}, err
	}
	if err := r.release(ctx, tx, order); err != nil {
		return Order{}, err
	}
	return markDelivered(ctx, tx, order, in)
}

func (r *Repository) cancelTx(ctx context.Context, tx pgx.Tx, customerID, orderID uuid.UUID, reason string) (Order, error) {
	order, err := lockOrder(ctx, tx, orderID)
	if err != nil {
		return Order{}, err
	}
	if err := customerCancel(order, customerID, reason); err != nil {
		return Order{}, err
	}
	if err := r.refund(ctx, tx, order); err != nil {
		return Order{}, err
	}
	return markCancelled(ctx, tx, order, customerID, reason)
}

func (r *Repository) release(ctx context.Context, tx pgx.Tx, order Order) error {
	if r.books == nil {
		return nil
	}
	payment, err := lockPayment(ctx, tx, order.ID)
	if err != nil {
		return err
	}
	if payment.Status != string(payments.StatusHeld) {
		return nil
	}
	return r.books.Release(ctx, tx, releaseInput(order, payment.ID))
}

func (r *Repository) refund(ctx context.Context, tx pgx.Tx, order Order) error {
	if r.books == nil {
		return nil
	}
	payment, err := lockPayment(ctx, tx, order.ID)
	if err != nil {
		return err
	}
	if payment.Status != string(payments.StatusHeld) {
		return nil
	}
	return r.books.Refund(ctx, tx, payments.RefundInput{
		OrderID: order.ID, PaymentID: payment.ID, CustomerID: order.CustomerID,
		Method: payments.Method(order.PaymentMethod), Total: order.Total,
	})
}

func releaseInput(order Order, paymentID uuid.UUID) payments.ReleaseInput {
	riderID := uuid.Nil
	if order.RiderID != nil {
		riderID = *order.RiderID
	}
	return payments.ReleaseInput{
		OrderID: order.ID, PaymentID: paymentID, CustomerID: order.CustomerID,
		MerchantID: order.MerchantID, RiderID: riderID, Method: payments.Method(order.PaymentMethod),
		ItemTotal: order.ItemTotal, DeliveryFee: order.DeliveryFee,
		Commission: order.Commission, RiderEarning: order.RiderEarning,
	}
}

func authorizeAdvance(order Order, in advance) error {
	if err := ownsParty(order, in); err != nil {
		return err
	}
	return transitionOrConflict(order, in.To)
}

func ownsParty(order Order, in advance) error {
	if mismatch(order.MerchantID, in.ExpectMerchant) {
		return apperror.Forbidden("merchant does not own this order")
	}
	if mismatch(order.RiderID, in.ExpectRider) {
		return apperror.Forbidden("rider is not assigned to this order")
	}
	return nil
}

func mismatch(have, want *uuid.UUID) bool {
	if want == nil {
		return false
	}
	return have == nil || *have != *want
}

func transitionOrConflict(order Order, to Status) error {
	if err := Transition(order.Type, order.Status, to); err != nil {
		return apperror.Conflict("order status cannot make that change")
	}
	return nil
}

func customerCancel(order Order, customerID uuid.UUID, reason string) error {
	if order.CustomerID != customerID {
		return apperror.Forbidden("order belongs to another customer")
	}
	if len(reason) < 1 || len(reason) > 200 {
		return apperror.Invalid("cancel_reason is invalid")
	}
	return transitionOrConflict(order, StatusCancelled)
}

func allowDeliver(order Order, in deliverInput) error {
	if mismatch(order.RiderID, &in.RiderID) {
		return apperror.Forbidden("rider is not assigned to this order")
	}
	if err := transitionOrConflict(order, StatusDelivered); err != nil {
		return err
	}
	return deliverProof(order, in)
}

func deliverProof(order Order, in deliverInput) error {
	if RequiresMerchant(order.Type) {
		return otpOK(order.DeliveryHash, in.OTP, in.HashKey)
	}
	return proofOK(order, in.Proof)
}

func acceptTx(ctx context.Context, tx pgx.Tx, riderID, orderID uuid.UUID) (Order, error) {
	order, err := lockOrder(ctx, tx, orderID)
	if err != nil {
		return Order{}, err
	}
	if err := transitionOrConflict(order, StatusAccepted); err != nil {
		return Order{}, err
	}
	if err := claimOffer(ctx, tx, order, riderID); err != nil {
		return Order{}, err
	}
	return order, nil
}

func rejectTx(ctx context.Context, tx pgx.Tx, riderID, orderID uuid.UUID) (Order, error) {
	order, err := lockOrder(ctx, tx, orderID)
	if err != nil {
		return Order{}, err
	}
	if order.Status != StatusRiderOffered {
		return Order{}, apperror.Conflict("order is not offered")
	}
	if err := setOffer(ctx, tx, orderID, riderID, "rejected"); err != nil {
		return Order{}, err
	}
	return order, nil
}

func claimOffer(ctx context.Context, tx pgx.Tx, order Order, riderID uuid.UUID) error {
	if err := setOffer(ctx, tx, order.ID, riderID, "accepted"); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
UPDATE orders SET status = 'accepted', rider_id = $2
WHERE id = $1 AND status = 'rider_offered' AND rider_id IS NULL`, order.ID, riderID)
	if err != nil {
		return fmt.Errorf("orders: accept: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperror.Conflict("offer already taken")
	}
	return insertStatusEvent(ctx, tx, order.ID, string(order.Status), string(StatusAccepted), string(domain.RoleRider), riderID, "")
}

func setOffer(ctx context.Context, tx pgx.Tx, orderID, riderID uuid.UUID, response string) error {
	tag, err := tx.Exec(ctx, `
UPDATE order_offers SET response = $3, responded_at = now()
WHERE order_id = $1 AND rider_id = $2 AND response IS NULL`, orderID, riderID, response)
	if err != nil {
		return fmt.Errorf("orders: offer: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

func applyStatus(ctx context.Context, tx pgx.Tx, order Order, to Status, role string, actorID uuid.UUID, note string) error {
	tag, err := tx.Exec(ctx, `UPDATE orders SET status = $3 WHERE id = $1 AND status = $2`, order.ID, string(order.Status), string(to))
	if err != nil {
		return fmt.Errorf("orders: status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperror.Conflict("order status changed")
	}
	return insertStatusEvent(ctx, tx, order.ID, string(order.Status), string(to), role, actorID, note)
}

func markDelivered(ctx context.Context, tx pgx.Tx, order Order, in deliverInput) (Order, error) {
	tag, err := tx.Exec(ctx, `
UPDATE orders SET status = 'delivered', delivered_at = now(), proof_photo_key = $2
WHERE id = $1 AND status = $3`, order.ID, textArg(in.Proof), string(order.Status))
	if err != nil {
		return Order{}, fmt.Errorf("orders: deliver: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return Order{}, apperror.Conflict("order status changed")
	}
	if err := insertStatusEvent(ctx, tx, order.ID, string(order.Status), string(StatusDelivered), string(domain.RoleRider), in.RiderID, ""); err != nil {
		return Order{}, err
	}
	order.Status = StatusDelivered
	return order, nil
}

func markCancelled(ctx context.Context, tx pgx.Tx, order Order, customerID uuid.UUID, reason string) (Order, error) {
	tag, err := tx.Exec(ctx, `
UPDATE orders SET status = 'cancelled', cancel_reason = $2
WHERE id = $1 AND status = $3`, order.ID, reason, string(order.Status))
	if err != nil {
		return Order{}, fmt.Errorf("orders: cancel: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return Order{}, apperror.Conflict("order status changed")
	}
	if err := insertStatusEvent(ctx, tx, order.ID, string(order.Status), string(StatusCancelled), string(domain.RoleCustomer), customerID, reason); err != nil {
		return Order{}, err
	}
	order.Status = StatusCancelled
	return order, nil
}

func lockOrder(ctx context.Context, tx pgx.Tx, orderID uuid.UUID) (Order, error) {
	order, err := scanOrder(tx.QueryRow(ctx, lockOrderSQL, orderID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, apperror.ErrNotFound
	}
	if err != nil {
		return Order{}, fmt.Errorf("orders: lock: %w", err)
	}
	return order, nil
}

func lockPayment(ctx context.Context, tx pgx.Tx, orderID uuid.UUID) (paymentRow, error) {
	var row paymentRow
	err := tx.QueryRow(ctx, lockPaymentSQL, orderID).Scan(&row.ID, &row.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return paymentRow{}, apperror.ErrNotFound
	}
	if err != nil {
		return paymentRow{}, fmt.Errorf("orders: payment: %w", err)
	}
	return row, nil
}

func insertStatusEvent(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, from, to, role string, actorID uuid.UUID, note string) error {
	_, err := tx.Exec(ctx, `
INSERT INTO order_events (order_id, from_status, to_status, actor_role, actor_id, note)
VALUES ($1, $2, $3, $4, $5, $6)`, orderID, from, to, role, actorID, textArg(note))
	if err != nil {
		return fmt.Errorf("orders: event: %w", err)
	}
	return nil
}
