package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yourusername/ghartak-backend/internal/platform/database"
	"github.com/yourusername/ghartak-backend/internal/platform/pii"
)

const ensureDemoCustomerSQL = `
INSERT INTO users (id, phone_ciphertext, phone_lookup, name, phone_verified, preferred_order_types)
VALUES ($1, $2, $3, $4, true, $5)
ON CONFLICT (phone_lookup) WHERE phone_lookup IS NOT NULL DO NOTHING`

const ensureDemoAddressSQL = `
INSERT INTO addresses (user_id, label, lat, lng, address_text)
SELECT $1, $2, $3, $4, $5
WHERE NOT EXISTS (
    SELECT 1 FROM addresses WHERE user_id = $1 AND label = $2
)`

// SeedFatehJangCustomers creates demo customers with home pins in Fateh Jang (development).
func (s *Service) SeedFatehJangCustomers(ctx context.Context, pool *pgxpool.Pool) error {
	for _, spot := range fatehJangCustomers() {
		if err := s.ensureDemoCustomer(ctx, pool, spot); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ensureDemoCustomer(ctx context.Context, pool *pgxpool.Pool, spot fatehCustomer) error {
	cipher, lookup, err := pii.SealPhone(s.piiKey, s.hashKey, spot.phone)
	if err != nil {
		return err
	}
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if _, err := pool.Exec(ctx, ensureDemoCustomerSQL, spot.id, cipher, lookup, spot.name, spot.prefs); err != nil {
		return fmt.Errorf("auth: seed customer: %w", err)
	}
	var id uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM users WHERE phone_lookup = $1 AND deleted_at IS NULL`, lookup).Scan(&id); err != nil {
		return fmt.Errorf("auth: seed customer lookup: %w", err)
	}
	if _, err := pool.Exec(ctx, ensureDemoAddressSQL, id, "home", spot.lat, spot.lng, spot.address); err != nil {
		return fmt.Errorf("auth: seed address: %w", err)
	}
	return nil
}

type fatehCustomer struct {
	id      uuid.UUID
	phone   string
	name    string
	lat     float64
	lng     float64
	address string
	prefs   []string
}

func fatehJangCustomers() []fatehCustomer {
	return []fatehCustomer{
		{
			id: uuid.MustParse("44444444-4444-4444-8444-444444444401"),
			phone: "+923001111001", name: "Ayesha Khan",
			lat: 33.568500, lng: 72.643500,
			address: "House near Railway Station Rd, Fateh Jang",
			prefs: []string{"food"},
		},
		{
			id: uuid.MustParse("44444444-4444-4444-8444-444444444402"),
			phone: "+923001111002", name: "Ali Hassan",
			lat: 33.571000, lng: 72.647000,
			address: "Near New Adda, Fateh Jang",
			prefs: []string{"food", "mart"},
		},
		{
			id: uuid.MustParse("44444444-4444-4444-8444-444444444403"),
			phone: "+923001111003", name: "Sana Malik",
			lat: 33.575000, lng: 72.650000,
			address: "Fateh Jang Rd, block B",
			prefs: []string{"food", "courier"},
		},
		{
			id: uuid.MustParse("44444444-4444-4444-8444-444444444404"),
			phone: "+923001111004", name: "Omar Farooq",
			lat: 33.566000, lng: 72.640500,
			address: "Kohat Rd colony, Fateh Jang",
			prefs: []string{"mart"},
		},
		{
			id: uuid.MustParse("44444444-4444-4444-8444-444444444405"),
			phone: "+923001111005", name: "Hira Bibi",
			lat: 33.569200, lng: 72.645000,
			address: "Main bazaar side street, Fateh Jang",
			prefs: []string{"food", "mart", "courier"},
		},
	}
}
