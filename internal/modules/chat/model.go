package chat

import (
	"time"

	"github.com/google/uuid"
)

type Message struct {
	ID         uuid.UUID `json:"id"`
	OrderID    uuid.UUID `json:"order_id"`
	SenderRole string    `json:"sender_role"`
	SenderID   uuid.UUID `json:"sender_id"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
}
