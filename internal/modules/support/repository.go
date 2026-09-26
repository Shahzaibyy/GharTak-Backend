package support

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

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Rate(ctx context.Context, in RatingInput) error {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("support: begin: %w", err)
	}
	if err := rateTx(ctx, tx, in); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("support: commit: %w", err)
	}
	return nil
}

func rateTx(ctx context.Context, tx pgx.Tx, in RatingInput) error {
	party, err := lockParty(ctx, tx, in.OrderID)
	if err != nil {
		return err
	}
	if err := allowRate(party, in); err != nil {
		return err
	}
	if err := insertRating(ctx, tx, in); err != nil {
		return err
	}
	return refreshRider(ctx, tx, in)
}

func (r *Repository) Open(ctx context.Context, customerID, orderID uuid.UUID, body string) (Ticket, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Ticket{}, fmt.Errorf("support: begin: %w", err)
	}
	ticket, err := openTx(ctx, tx, customerID, orderID, body)
	if err != nil {
		_ = tx.Rollback(ctx)
		return Ticket{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Ticket{}, fmt.Errorf("support: commit: %w", err)
	}
	return ticket, nil
}

func openTx(ctx context.Context, tx pgx.Tx, customerID, orderID uuid.UUID, body string) (Ticket, error) {
	if err := customerOrder(ctx, tx, customerID, orderID); err != nil {
		return Ticket{}, err
	}
	ticket, err := insertTicket(ctx, tx, customerID, orderID)
	if err != nil {
		return Ticket{}, err
	}
	_, err = insertMessage(ctx, tx, ticket.ID, "customer", customerID, body)
	if err != nil {
		return Ticket{}, err
	}
	return ticket, nil
}

func (r *Repository) List(ctx context.Context, customerID uuid.UUID) ([]Ticket, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	rows, err := r.pool.Query(ctx, `
SELECT id, order_id, customer_id, status, created_at
FROM support_tickets
WHERE customer_id = $1
ORDER BY created_at DESC
LIMIT 20`, customerID)
	if err != nil {
		return nil, fmt.Errorf("support: tickets: %w", err)
	}
	defer rows.Close()
	return scanTickets(rows)
}

func (r *Repository) Get(ctx context.Context, customerID, ticketID uuid.UUID) (Ticket, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var ticket Ticket
	err := r.pool.QueryRow(ctx, `
SELECT id, order_id, customer_id, status, created_at
FROM support_tickets WHERE id = $1 AND customer_id = $2`, ticketID, customerID).Scan(
		&ticket.ID, &ticket.OrderID, &ticket.CustomerID, &ticket.Status, &ticket.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Ticket{}, apperror.ErrNotFound
	}
	if err != nil {
		return Ticket{}, fmt.Errorf("support: ticket: %w", err)
	}
	return ticket, nil
}

func (r *Repository) Messages(ctx context.Context, customerID, ticketID uuid.UUID) ([]Message, error) {
	if _, err := r.Get(ctx, customerID, ticketID); err != nil {
		return nil, err
	}
	return r.listMessages(ctx, ticketID)
}

func (r *Repository) Reply(ctx context.Context, customerID, ticketID uuid.UUID, role string, senderID uuid.UUID, body string) (Message, error) {
	ticket, err := r.Get(ctx, customerID, ticketID)
	if err != nil {
		return Message{}, err
	}
	if ticket.Status != "open" {
		return Message{}, apperror.Conflict("ticket is closed")
	}
	return r.addMessage(ctx, ticketID, role, senderID, body)
}

func (r *Repository) listMessages(ctx context.Context, ticketID uuid.UUID) ([]Message, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	rows, err := r.pool.Query(ctx, `
SELECT id, ticket_id, sender_role, sender_id, body, created_at
FROM ticket_messages WHERE ticket_id = $1 ORDER BY created_at LIMIT 50`, ticketID)
	if err != nil {
		return nil, fmt.Errorf("support: messages: %w", err)
	}
	defer rows.Close()
	return scanMessages(rows)
}

func (r *Repository) addMessage(ctx context.Context, ticketID uuid.UUID, role string, senderID uuid.UUID, body string) (Message, error) {
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	msg, err := insertMessage(ctx, r.pool, ticketID, role, senderID, body)
	if err != nil {
		return Message{}, err
	}
	_, err = r.pool.Exec(ctx, `UPDATE support_tickets SET updated_at = now() WHERE id = $1`, ticketID)
	if err != nil {
		return Message{}, fmt.Errorf("support: ticket touch: %w", err)
	}
	return msg, nil
}

type queryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func lockParty(ctx context.Context, tx pgx.Tx, orderID uuid.UUID) (orderParty, error) {
	var party orderParty
	err := tx.QueryRow(ctx, `
SELECT status, customer_id, rider_id, merchant_id FROM orders WHERE id = $1 FOR UPDATE`, orderID).Scan(
		&party.Status, &party.CustomerID, &party.RiderID, &party.MerchantID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return orderParty{}, apperror.ErrNotFound
	}
	if err != nil {
		return orderParty{}, fmt.Errorf("support: order: %w", err)
	}
	return party, nil
}

func insertRating(ctx context.Context, tx pgx.Tx, in RatingInput) error {
	_, err := tx.Exec(ctx, `
INSERT INTO ratings (order_id, rater_role, rater_id, ratee_role, ratee_id, score, comment)
VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		in.OrderID, in.RaterRole, in.RaterID, in.RateeRole, in.RateeID, in.Score, commentArg(in.Comment),
	)
	if isReject(err) {
		return apperror.Conflict("rating already exists")
	}
	if err != nil {
		return fmt.Errorf("support: rating: %w", err)
	}
	return nil
}

func refreshRider(ctx context.Context, tx pgx.Tx, in RatingInput) error {
	if in.RateeRole != "rider" {
		return nil
	}
	_, err := tx.Exec(ctx, `
UPDATE riders SET
    rating = (SELECT ROUND(AVG(score)::numeric, 2) FROM ratings WHERE ratee_id = $1 AND ratee_role = 'rider'),
    rating_count = (SELECT COUNT(*) FROM ratings WHERE ratee_id = $1 AND ratee_role = 'rider')
WHERE id = $1`, in.RateeID)
	if err != nil {
		return fmt.Errorf("support: rider rating: %w", err)
	}
	return nil
}

func customerOrder(ctx context.Context, tx pgx.Tx, customerID, orderID uuid.UUID) error {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM orders WHERE id = $1 AND customer_id = $2`, orderID, customerID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperror.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("support: order: %w", err)
	}
	return nil
}

func insertTicket(ctx context.Context, tx pgx.Tx, customerID, orderID uuid.UUID) (Ticket, error) {
	var ticket Ticket
	err := tx.QueryRow(ctx, `
INSERT INTO support_tickets (order_id, customer_id) VALUES ($1, $2)
RETURNING id, order_id, customer_id, status, created_at`, orderID, customerID).Scan(
		&ticket.ID, &ticket.OrderID, &ticket.CustomerID, &ticket.Status, &ticket.CreatedAt,
	)
	if err != nil {
		return Ticket{}, fmt.Errorf("support: ticket: %w", err)
	}
	return ticket, nil
}

func insertMessage(ctx context.Context, q queryer, ticketID uuid.UUID, role string, senderID uuid.UUID, body string) (Message, error) {
	var msg Message
	err := q.QueryRow(ctx, `
INSERT INTO ticket_messages (ticket_id, sender_role, sender_id, body)
VALUES ($1, $2, $3, $4)
RETURNING id, ticket_id, sender_role, sender_id, body, created_at`, ticketID, role, senderID, body).Scan(
		&msg.ID, &msg.TicketID, &msg.SenderRole, &msg.SenderID, &msg.Body, &msg.CreatedAt,
	)
	if err != nil {
		return Message{}, fmt.Errorf("support: message: %w", err)
	}
	return msg, nil
}

func scanTickets(rows pgx.Rows) ([]Ticket, error) {
	out := make([]Ticket, 0)
	for rows.Next() {
		ticket, err := scanTicket(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ticket)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("support: tickets: %w", err)
	}
	return out, nil
}

func scanTicket(row pgx.Row) (Ticket, error) {
	var ticket Ticket
	err := row.Scan(&ticket.ID, &ticket.OrderID, &ticket.CustomerID, &ticket.Status, &ticket.CreatedAt)
	if err != nil {
		return Ticket{}, fmt.Errorf("support: tickets: %w", err)
	}
	return ticket, nil
}

func scanMessages(rows pgx.Rows) ([]Message, error) {
	out := make([]Message, 0)
	for rows.Next() {
		msg, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("support: messages: %w", err)
	}
	return out, nil
}

func scanMessage(row pgx.Row) (Message, error) {
	var msg Message
	err := row.Scan(&msg.ID, &msg.TicketID, &msg.SenderRole, &msg.SenderID, &msg.Body, &msg.CreatedAt)
	if err != nil {
		return Message{}, fmt.Errorf("support: messages: %w", err)
	}
	return msg, nil
}

func commentArg(raw string) any {
	if raw == "" {
		return nil
	}
	return raw
}

func isReject(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505" || pgErr.Code == "23514"
}
