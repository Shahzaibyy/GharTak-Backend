package merchants

import (
	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/platform/money"
)

type Merchant struct {
	ID                 uuid.UUID `json:"id"`
	Name               string    `json:"name"`
	Category           Category  `json:"category"`
	ZoneID             uuid.UUID `json:"zone_id"`
	Lat                float64   `json:"lat"`
	Lng                float64   `json:"lng"`
	AddressText        string    `json:"address_text"`
	PhotoKey           *string   `json:"photo_key"`
	CommissionRate     string    `json:"commission_rate"`
	VerificationStatus string    `json:"verification_status"`
}

type CatalogItem struct {
	ID          uuid.UUID   `json:"id"`
	MerchantID  uuid.UUID   `json:"merchant_id"`
	Name        string      `json:"name"`
	Description *string     `json:"description"`
	Price       money.Money `json:"price"`
	IsAvailable bool        `json:"is_available"`
	PhotoKey    *string     `json:"photo_key"`
}

type RegisterInput struct {
	Phone       string
	OwnerName   string
	Name        string
	Category    Category
	ZoneID      uuid.UUID
	Lat         float64
	Lng         float64
	AddressText string
}

type CatalogInput struct {
	Name        string
	Description string
	Price       money.Money
	IsAvailable bool
	PhotoKey    string
}

type Pin struct {
	ID             uuid.UUID
	Lat            float64
	Lng            float64
	AddressText    string
	CommissionRate string
}

type Price struct {
	ID    uuid.UUID
	Name  string
	Price money.Money
}
