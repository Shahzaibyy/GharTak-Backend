package riders

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yourusername/ghartak-backend/internal/platform/database"
	"github.com/yourusername/ghartak-backend/internal/platform/domain"
	"github.com/yourusername/ghartak-backend/internal/platform/pii"
)

var fatehJangZoneID = uuid.MustParse("11111111-1111-4111-8111-111111111104")

const ensureDemoRiderSQL = `
INSERT INTO riders (
    id, phone_ciphertext, phone_lookup, name, cnic_ciphertext, cnic_lookup,
    vehicle_type, vehicle_reg, license_number, zone_id, verification_status,
    orientation_status, onboarding_step, application_received_at, is_online,
    cnic_front_object_key, cnic_back_object_key, selfie_object_key, license_object_key, cnic_object_key
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, 'approved',
    'booked', 'submitted', now(), true,
    $11, $12, $13, $14, $11
)
ON CONFLICT (phone_lookup) DO NOTHING`

// SeedFatehJang creates approved online riders around Fateh Jang restaurants (development).
func (s *Service) SeedFatehJang(ctx context.Context, pool *pgxpool.Pool) error {
	for _, spot := range fatehJangRiders() {
		if err := s.ensureDemoRider(ctx, pool, spot); err != nil {
			return err
		}
		if err := s.seedPresence(ctx, spot); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ensureDemoRider(ctx context.Context, pool *pgxpool.Pool, spot fatehRider) error {
	phoneCipher, phoneLookup, err := pii.SealPhone(s.piiKey, s.hashKey, spot.phone)
	if err != nil {
		return err
	}
	cnicCipher, cnicLookup, err := pii.SealCNIC(s.piiKey, s.hashKey, spot.cnic)
	if err != nil {
		return err
	}
	ctx, cancel := database.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err = pool.Exec(ctx, ensureDemoRiderSQL,
		spot.id, phoneCipher, phoneLookup, spot.name, cnicCipher, cnicLookup,
		string(domain.VehicleMotorcycle), spot.plate, spot.licence, fatehJangZoneID,
		"demo/cnic-front/"+spot.id.String(),
		"demo/cnic-back/"+spot.id.String(),
		"demo/selfie/"+spot.id.String(),
		"demo/licence/"+spot.id.String(),
	)
	if err != nil {
		return fmt.Errorf("riders: seed fateh jang: %w", err)
	}
	return nil
}

func (s *Service) seedPresence(ctx context.Context, spot fatehRider) error {
	if s.presence == nil {
		return nil
	}
	return s.presence.Online(ctx, fatehJangZoneID, spot.id, spot.lat, spot.lng)
}

type fatehRider struct {
	id      uuid.UUID
	phone   string
	name    string
	cnic    string
	plate   string
	licence string
	lat     float64
	lng     float64
}

func fatehJangRiders() []fatehRider {
	return []fatehRider{
		{
			id: uuid.MustParse("33333333-3333-4333-8333-333333333301"),
			phone: "+923002222001", name: "Usman Ali", cnic: "37101-1111111-1",
			plate: "ATK-1001", licence: "PJ-FJ-1001",
			lat: 33.570100, lng: 72.646400, // near Spanish Pizza / New Adda
		},
		{
			id: uuid.MustParse("33333333-3333-4333-8333-333333333302"),
			phone: "+923002222002", name: "Bilal Khan", cnic: "37101-2222222-2",
			plate: "ATK-1002", licence: "PJ-FJ-1002",
			lat: 33.567600, lng: 72.641900, // near Bismillah / Attock Chowk
		},
		{
			id: uuid.MustParse("33333333-3333-4333-8333-333333333303"),
			phone: "+923002222003", name: "Hamza Iqbal", cnic: "37101-3333333-3",
			plate: "ATK-1003", licence: "PJ-FJ-1003",
			lat: 33.578200, lng: 72.653000, // near Mehria
		},
		{
			id: uuid.MustParse("33333333-3333-4333-8333-333333333304"),
			phone: "+923002222004", name: "Saad Raza", cnic: "37101-4444444-4",
			plate: "ATK-1004", licence: "PJ-FJ-1004",
			lat: 33.571400, lng: 72.648800, // near Sawat Naan
		},
		{
			id: uuid.MustParse("33333333-3333-4333-8333-333333333305"),
			phone: "+923002222005", name: "Farhan Malik", cnic: "37101-5555555-5",
			plate: "ATK-1005", licence: "PJ-FJ-1005",
			lat: 33.565500, lng: 72.641600, // near Talha Hotel
		},
	}
}
