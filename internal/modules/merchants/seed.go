package merchants

import (
	"context"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/platform/money"
	"github.com/yourusername/ghartak-backend/internal/platform/pii"
)

const demoPhone = "+923000000001"

var demoMerchantID = uuid.MustParse("22222222-2222-4222-8222-222222222201")

var attockZoneID = uuid.MustParse("11111111-1111-4111-8111-111111111101")

func (s *Service) SeedDemo(ctx context.Context) error {
	ciphertext, lookup, err := pii.SealPhone(s.piiKey, s.hashKey, demoPhone)
	if err != nil {
		return err
	}
	id, err := s.store.EnsureDemo(ctx, demoMerchant(ciphertext, lookup))
	if err != nil {
		return err
	}
	return s.seedMenu(ctx, id)
}

func (s *Service) seedMenu(ctx context.Context, merchantID uuid.UUID) error {
	count, err := s.store.CatalogCount(ctx, merchantID)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return s.insertDemoItems(ctx, merchantID)
}

func (s *Service) insertDemoItems(ctx context.Context, merchantID uuid.UUID) error {
	for _, item := range demoMenu() {
		if _, err := s.store.InsertItem(ctx, merchantID, item); err != nil {
			return err
		}
	}
	return nil
}

func demoMerchant(ciphertext, lookup string) demoRow {
	return demoRow{
		ID: demoMerchantID, Ciphertext: ciphertext, Lookup: lookup,
		OwnerName: "GharTak", Name: "GharTak Demo Kitchen", Category: CategoryRestaurant,
		ZoneID: attockZoneID, Lat: 33.773, Lng: 72.360,
		AddressText: "Demo kitchen, Attock City", Commission: "18.00",
	}
}

func demoMenu() []catalogRow {
	return []catalogRow{
		{Name: "Chicken Biryani", Price: mustPrice("450.00"), IsAvailable: true},
		{Name: "Seekh Kabab", Price: mustPrice("250.00"), IsAvailable: true},
	}
}

func mustPrice(raw string) string {
	parsed, err := money.Parse(raw)
	if err != nil {
		return raw
	}
	return parsed.String()
}
