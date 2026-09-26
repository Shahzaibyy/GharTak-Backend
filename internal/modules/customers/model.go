package customers

import (
	"time"

	"github.com/google/uuid"
)

type Address struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	Label       string    `json:"label"`
	Lat         float64   `json:"lat"`
	Lng         float64   `json:"lng"`
	AddressText string    `json:"address_text"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type AddressInput struct {
	Label       string
	Lat         float64
	Lng         float64
	AddressText string
}
