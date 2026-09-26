package orders

import (
	"time"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/platform/money"
)

type ItemInput struct {
	CatalogItemID uuid.UUID
	Quantity      int
}

type PlaceInput struct {
	Type            Type
	ZoneID          uuid.UUID
	MerchantID      *uuid.UUID
	PickupLat       float64
	PickupLng       float64
	PickupAddress   string
	DropLat         float64
	DropLng         float64
	DropAddress     string
	Description     string
	Effort          *EffortTier
	Items           []ItemInput
	PaymentMethod   string
	ClientRequestID string
}

type OrderItem struct {
	CatalogItemID *uuid.UUID  `json:"catalog_item_id"`
	Name          string      `json:"item_name"`
	Quantity      int         `json:"quantity"`
	Price         money.Money `json:"price_at_order"`
}

type Order struct {
	ID              uuid.UUID   `json:"id"`
	Type            Type        `json:"type"`
	CustomerID      uuid.UUID   `json:"customer_id"`
	MerchantID      *uuid.UUID  `json:"merchant_id"`
	ZoneID          uuid.UUID   `json:"zone_id"`
	Status          Status      `json:"status"`
	PickupLat       float64     `json:"pickup_lat"`
	PickupLng       float64     `json:"pickup_lng"`
	DropLat         float64     `json:"drop_lat"`
	DropLng         float64     `json:"drop_lng"`
	PickupAddress   string      `json:"pickup_address"`
	DropAddress     string      `json:"drop_address"`
	Description     *string     `json:"description"`
	EffortTier      *string     `json:"effort_tier"`
	PaymentMethod   string      `json:"payment_method"`
	ClientRequestID string      `json:"client_request_id"`
	RiderID         *uuid.UUID  `json:"rider_id"`
	DeliveryOTP     string      `json:"delivery_otp,omitempty"`
	CreatedAt       time.Time   `json:"created_at"`
	Items           []OrderItem `json:"items"`
	DeliveryHash    string      `json:"-"`
	Priced
}

type draft struct {
	CustomerID    uuid.UUID
	Input         PlaceInput
	PickupLat     float64
	PickupLng     float64
	PickupAddress string
	Items         []OrderItem
	Price         Priced
	DeliveryOTP   string
	DeliveryHash  string
}

type Party struct {
	OrderID    uuid.UUID
	Type       Type
	Status     Status
	CustomerID uuid.UUID
	RiderID    *uuid.UUID
	MerchantID *uuid.UUID
	ZoneID     uuid.UUID
	PickupLat  float64
	PickupLng  float64
}

func (p Party) Allows(role string, id uuid.UUID) bool {
	check, ok := partyChecks[role]
	if !ok {
		return false
	}
	return check(p, id)
}

var partyChecks = map[string]func(Party, uuid.UUID) bool{
	"customer": func(p Party, id uuid.UUID) bool { return p.CustomerID == id },
	"rider":    func(p Party, id uuid.UUID) bool { return p.RiderID != nil && *p.RiderID == id },
	"merchant": func(p Party, id uuid.UUID) bool { return p.MerchantID != nil && *p.MerchantID == id },
}
