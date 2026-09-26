package payments

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

const insertLegSQL = `
INSERT INTO ledger_entries (
    journal_id, order_id, payment_id, account_kind, account_id, direction, amount, entry_type, idempotency_key
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

const paymentStatusSQL = `
UPDATE payments SET status = $2 WHERE id = $1 AND status <> $2`

const gatewayRefSQL = `
UPDATE payments SET gateway_ref = $2 WHERE id = $1 AND gateway_ref IS NULL`

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Hold(ctx context.Context, tx pgx.Tx, in HoldInput) error {
	journal, err := HoldJournal(in)
	if err != nil {
		return err
	}
	if err := r.Post(ctx, tx, journal); err != nil {
		return err
	}
	return stampGateway(ctx, tx, in)
}

func (r *Repository) Release(ctx context.Context, tx pgx.Tx, in ReleaseInput) error {
	journal, err := ReleaseJournal(in)
	if err != nil {
		return err
	}
	if err := r.Post(ctx, tx, journal); err != nil {
		return err
	}
	return setPaymentStatus(ctx, tx, in.PaymentID, StatusReleased)
}

func (r *Repository) Refund(ctx context.Context, tx pgx.Tx, in RefundInput) error {
	journal, err := RefundJournal(in)
	if err != nil {
		return err
	}
	if err := r.Post(ctx, tx, journal); err != nil {
		return err
	}
	return setPaymentStatus(ctx, tx, in.PaymentID, StatusRefunded)
}

func (r *Repository) CreditWallet(ctx context.Context, in AdjustInput) error {
	journal, err := AdjustJournal(in)
	if err != nil {
		return err
	}
	return r.inTx(ctx, journal)
}

func (r *Repository) SettleCash(ctx context.Context, in SettleInput) error {
	journal, err := SettleJournal(in)
	if err != nil {
		return err
	}
	return r.inTx(ctx, journal)
}

func (r *Repository) ApplyWebhook(ctx context.Context, ref string, status Status) error {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	current, err := r.statusByRef(ctx, ref)
	if err != nil {
		return err
	}
	if current == status {
		return nil
	}
	return r.writeWebhook(ctx, ref, status)
}

func (r *Repository) writeWebhook(ctx context.Context, ref string, status Status) error {
	tag, err := r.pool.Exec(ctx, `UPDATE payments SET status = $2 WHERE gateway_ref = $1`, ref, string(status))
	if err != nil {
		return fmt.Errorf("payments: webhook: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

func (r *Repository) Post(ctx context.Context, tx pgx.Tx, journal Journal) error {
	if len(journal.Legs) == 0 {
		return nil
	}
	fresh, err := freshJournal(ctx, tx, journal)
	if err != nil || !fresh {
		return err
	}
	return writeJournal(ctx, tx, journal)
}

func freshJournal(ctx context.Context, tx pgx.Tx, journal Journal) (bool, error) {
	if err := AssertBalanced(journal); err != nil {
		return false, err
	}
	posted, err := postedKey(ctx, tx, journal.Legs[0].Key)
	if err != nil {
		return false, err
	}
	return !posted, nil
}

func writeJournal(ctx context.Context, tx pgx.Tx, journal Journal) error {
	if err := applyCaches(ctx, tx, journal); err != nil {
		return err
	}
	return insertLegs(ctx, tx, journal)
}

func (r *Repository) inTx(ctx context.Context, journal Journal) error {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("payments: begin: %w", err)
	}
	if err := r.Post(ctx, tx, journal); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("payments: commit: %w", err)
	}
	return nil
}

func (r *Repository) statusByRef(ctx context.Context, ref string) (Status, error) {
	var status string
	err := r.pool.QueryRow(ctx, `SELECT status FROM payments WHERE gateway_ref = $1`, ref).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apperror.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("payments: gateway ref: %w", err)
	}
	return Status(status), nil
}

func stampGateway(ctx context.Context, tx pgx.Tx, in HoldInput) error {
	ref, ok := GatewayStubRef(in.Method, in.OrderID)
	if !ok {
		return nil
	}
	_, err := tx.Exec(ctx, gatewayRefSQL, in.PaymentID, ref)
	if err != nil {
		return fmt.Errorf("payments: gateway ref: %w", err)
	}
	return nil
}

func setPaymentStatus(ctx context.Context, tx pgx.Tx, paymentID uuid.UUID, status Status) error {
	_, err := tx.Exec(ctx, paymentStatusSQL, paymentID, string(status))
	if err != nil {
		return fmt.Errorf("payments: status: %w", err)
	}
	return nil
}

func postedKey(ctx context.Context, tx pgx.Tx, key string) (bool, error) {
	var posted bool
	err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM ledger_entries WHERE idempotency_key = $1)`, key).Scan(&posted)
	if err != nil {
		return false, fmt.Errorf("payments: idempotency: %w", err)
	}
	return posted, nil
}

func insertLegs(ctx context.Context, tx pgx.Tx, journal Journal) error {
	for _, item := range journal.Legs {
		if err := insertLeg(ctx, tx, journal, item); err != nil {
			return err
		}
	}
	return nil
}

func insertLeg(ctx context.Context, tx pgx.Tx, journal Journal, item Leg) error {
	_, err := tx.Exec(ctx, insertLegSQL,
		journal.ID, uuidArg(journal.OrderID), uuidArg(journal.PaymentID),
		string(item.Kind), item.AccountID, string(item.Direction), item.Amount.String(),
		string(journal.Type), item.Key,
	)
	if err != nil {
		return fmt.Errorf("payments: insert leg: %w", err)
	}
	return nil
}

func applyCaches(ctx context.Context, tx pgx.Tx, journal Journal) error {
	for _, item := range journal.Legs {
		if err := applyCache(ctx, tx, item); err != nil {
			return err
		}
	}
	return nil
}

type cacheSpec struct {
	query balanceSQL
	asset bool
	low   string
}

var cacheSpecs = map[AccountKind]cacheSpec{
	KindCustomerWallet: {query: walletSQL, low: "wallet balance is too low"},
	KindRiderCashOwed:  {query: cashSQL, asset: true, low: "cash owed is too low"},
}

func applyCache(ctx context.Context, tx pgx.Tx, item Leg) error {
	spec, ok := cacheSpecs[item.Kind]
	if !ok {
		return nil
	}
	return moveBalance(ctx, tx, item, spec)
}

type balanceSQL struct {
	lock   string
	update string
}

var walletSQL = balanceSQL{
	lock:   `SELECT wallet_balance::text FROM users WHERE id = $1 FOR UPDATE`,
	update: `UPDATE users SET wallet_balance = $2 WHERE id = $1`,
}

var cashSQL = balanceSQL{
	lock:   `SELECT cash_owed::text FROM riders WHERE id = $1 FOR UPDATE`,
	update: `UPDATE riders SET cash_owed = $2 WHERE id = $1`,
}

func moveBalance(ctx context.Context, tx pgx.Tx, item Leg, spec cacheSpec) error {
	current, err := lockBalance(ctx, tx, spec.query.lock, item.AccountID)
	if err != nil {
		return err
	}
	next, err := moved(current, item, spec.asset)
	if err != nil {
		return err
	}
	return writeBalance(ctx, tx, spec, item.AccountID, next)
}

func writeBalance(ctx context.Context, tx pgx.Tx, spec cacheSpec, id uuid.UUID, next money.Money) error {
	if next.IsNegative() {
		return apperror.Conflict(spec.low)
	}
	_, err := tx.Exec(ctx, spec.query.update, id, next.String())
	if err != nil {
		return fmt.Errorf("payments: balance: %w", err)
	}
	return nil
}

func lockBalance(ctx context.Context, tx pgx.Tx, query string, id uuid.UUID) (money.Money, error) {
	var raw string
	err := tx.QueryRow(ctx, query, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return money.Money{}, apperror.ErrNotFound
	}
	if err != nil {
		return money.Money{}, fmt.Errorf("payments: lock balance: %w", err)
	}
	amount, err := money.Parse(raw)
	if err != nil {
		return money.Money{}, fmt.Errorf("payments: parse balance: %w", err)
	}
	return amount, nil
}

func moved(current money.Money, item Leg, asset bool) (money.Money, error) {
	if increases(item.Direction, asset) {
		return current.Add(item.Amount)
	}
	return current.Sub(item.Amount)
}

func increases(direction Direction, asset bool) bool {
	if asset {
		return direction == Debit
	}
	return direction == Credit
}

func uuidArg(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}
