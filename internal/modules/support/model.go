package support

import (
	"time"

	"github.com/google/uuid"
)

type RatingInput struct {
	OrderID   uuid.UUID
	RaterRole string
	RaterID   uuid.UUID
	RateeRole string
	RateeID   uuid.UUID
	Score     int
	Comment   string
}

type Ticket struct {
	ID         uuid.UUID `json:"id"`
	OrderID    uuid.UUID `json:"order_id"`
	CustomerID uuid.UUID `json:"customer_id"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

type Message struct {
	ID         uuid.UUID `json:"id"`
	TicketID   uuid.UUID `json:"ticket_id"`
	SenderRole string    `json:"sender_role"`
	SenderID   uuid.UUID `json:"sender_id"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
}

type orderParty struct {
	Status     string
	CustomerID uuid.UUID
	RiderID    *uuid.UUID
	MerchantID *uuid.UUID
}
