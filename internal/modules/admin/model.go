package admin

import (
	"time"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/platform/money"
)

type Zone struct {
	ID              uuid.UUID   `json:"id"`
	CityName        string      `json:"city_name"`
	Slug            string      `json:"slug"`
	BaseDeliveryFee money.Money `json:"base_delivery_fee"`
	PerKmRate       money.Money `json:"per_km_rate"`
	SurgeMultiplier string      `json:"surge_multiplier"`
	ServiceRadiusKm string      `json:"service_radius_km"`
	CenterLat       float64     `json:"center_lat"`
	CenterLng       float64     `json:"center_lng"`
	IsActive        bool        `json:"is_active"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
}

type ZoneSeed struct {
	ID       uuid.UUID
	CityName string
	Slug     string
	Active   bool
}

const (
	SeedBaseDeliveryFee = "60.00"
	SeedPerKmRate       = "18.00"
	SeedSurge           = "1.00"
	SeedRadiusKm        = "8.00"
)

func ZoneSeeds() []ZoneSeed {
	return []ZoneSeed{
		{uuid.MustParse("11111111-1111-4111-8111-111111111101"), "Attock City", "attock-city", true},
		{uuid.MustParse("11111111-1111-4111-8111-111111111102"), "Hasan Abdal", "hasan-abdal", true},
		{uuid.MustParse("11111111-1111-4111-8111-111111111103"), "Hazro", "hazro", false},
		{uuid.MustParse("11111111-1111-4111-8111-111111111104"), "Fateh Jang", "fateh-jang", false},
		{uuid.MustParse("11111111-1111-4111-8111-111111111105"), "Jand", "jand", false},
		{uuid.MustParse("11111111-1111-4111-8111-111111111106"), "Pindi Gheb", "pindi-gheb", false},
	}
}
