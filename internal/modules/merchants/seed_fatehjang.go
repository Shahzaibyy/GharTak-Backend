package merchants

import (
	"context"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/platform/pii"
)

// Fateh Jang zone (Attock district). Center ~33.5672, 72.6417 — 8 km radius.
var fatehJangZoneID = uuid.MustParse("11111111-1111-4111-8111-111111111104")

// SeedFatehJang inserts approved restaurants at real Fateh Jang map pins (development only).
func (s *Service) SeedFatehJang(ctx context.Context) error {
	for _, spot := range fatehJangMerchants() {
		ciphertext, lookup, err := pii.SealPhone(s.piiKey, s.hashKey, spot.phone)
		if err != nil {
			return err
		}
		id, err := s.store.EnsureDemo(ctx, demoRow{
			ID: spot.id, Ciphertext: ciphertext, Lookup: lookup,
			OwnerName: spot.owner, Name: spot.name, Category: CategoryRestaurant,
			ZoneID: fatehJangZoneID, Lat: spot.lat, Lng: spot.lng,
			AddressText: spot.address, Commission: "18.00",
		})
		if err != nil {
			return err
		}
		if err := s.seedMenuNamed(ctx, id, spot.menu); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) seedMenuNamed(ctx context.Context, merchantID uuid.UUID, items []catalogRow) error {
	count, err := s.store.CatalogCount(ctx, merchantID)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	for _, item := range items {
		if _, err := s.store.InsertItem(ctx, merchantID, item); err != nil {
			return err
		}
	}
	return nil
}

type fatehSpot struct {
	id      uuid.UUID
	phone   string
	owner   string
	name    string
	lat     float64
	lng     float64
	address string
	menu    []catalogRow
}

func fatehJangMerchants() []fatehSpot {
	return []fatehSpot{
		{
			id: uuid.MustParse("22222222-2222-4222-8222-222222222211"), phone: "+923003333001",
			owner: "Bismillah", name: "Bismillah Restaurant",
			lat: 33.567565, lng: 72.641856,
			address: "Attock Chowk, Railway Station Rd, Fateh Jang",
			menu: []catalogRow{
				{Name: "Chicken Karahi", Price: mustPrice("1200.00"), IsAvailable: true},
				{Name: "Daal Mash", Price: mustPrice("350.00"), IsAvailable: true},
				{Name: "Naan", Price: mustPrice("40.00"), IsAvailable: true},
			},
		},
		{
			id: uuid.MustParse("22222222-2222-4222-8222-222222222212"), phone: "+923003333002",
			owner: "Talha", name: "Talha Hotel",
			lat: 33.565425, lng: 72.641523,
			address: "Rawalpindi–Kohat Rd, Fateh Jang",
			menu: []catalogRow{
				{Name: "Beef Pulao", Price: mustPrice("450.00"), IsAvailable: true},
				{Name: "Chapli Kabab", Price: mustPrice("280.00"), IsAvailable: true},
			},
		},
		{
			id: uuid.MustParse("22222222-2222-4222-8222-222222222213"), phone: "+923003333003",
			owner: "Spanish Pizza", name: "Spanish Pizza Fateh Jang",
			lat: 33.570230, lng: 72.646564,
			address: "Main Rawalpindi–Kohat Rd, opposite New Adda, Fateh Jang",
			menu: []catalogRow{
				{Name: "Chicken Fajita Pizza", Price: mustPrice("999.00"), IsAvailable: true},
				{Name: "Zinger Burger", Price: mustPrice("450.00"), IsAvailable: true},
				{Name: "Fries", Price: mustPrice("200.00"), IsAvailable: true},
			},
		},
		{
			id: uuid.MustParse("22222222-2222-4222-8222-222222222214"), phone: "+923003333004",
			owner: "Ramzan", name: "Ramzan Hotel & Fast Foods",
			lat: 33.570048, lng: 72.646179,
			address: "Near New Adda, Fateh Jang",
			menu: []catalogRow{
				{Name: "Chicken Shawarma", Price: mustPrice("300.00"), IsAvailable: true},
				{Name: "Grill Burger", Price: mustPrice("420.00"), IsAvailable: true},
			},
		},
		{
			id: uuid.MustParse("22222222-2222-4222-8222-222222222215"), phone: "+923003333005",
			owner: "Sawat", name: "Sawat Naan Centre",
			lat: 33.571450, lng: 72.648878,
			address: "Fateh Jang Rd, Fateh Jang",
			menu: []catalogRow{
				{Name: "Roghni Naan", Price: mustPrice("80.00"), IsAvailable: true},
				{Name: "Chicken Tikka", Price: mustPrice("550.00"), IsAvailable: true},
				{Name: "Mutton Karahi (half)", Price: mustPrice("1800.00"), IsAvailable: true},
			},
		},
		{
			id: uuid.MustParse("22222222-2222-4222-8222-222222222216"), phone: "+923003333006",
			owner: "Mehria", name: "Mehria Hotel & Restaurant",
			lat: 33.578310, lng: 72.653180,
			address: "Fateh Jang Rd, Fateh Jang",
			menu: []catalogRow{
				{Name: "Chicken Biryani", Price: mustPrice("400.00"), IsAvailable: true},
				{Name: "Seekh Kabab Plate", Price: mustPrice("500.00"), IsAvailable: true},
			},
		},
		{
			id: uuid.MustParse("22222222-2222-4222-8222-222222222217"), phone: "+923003333007",
			owner: "Ramzan Pulao", name: "Ramzan Murgh Pulao",
			lat: 33.567918, lng: 72.643240,
			address: "Main Rawalpindi Road, Fateh Jang City",
			menu: []catalogRow{
				{Name: "Murgh Pulao", Price: mustPrice("380.00"), IsAvailable: true},
				{Name: "Raita", Price: mustPrice("80.00"), IsAvailable: true},
			},
		},
		{
			id: uuid.MustParse("22222222-2222-4222-8222-222222222218"), phone: "+923003333008",
			owner: "Abaseen", name: "Abaseen Restaurant",
			lat: 33.593820, lng: 72.698970,
			address: "01 Attock Rd, Fateh Jang",
			menu: []catalogRow{
				{Name: "Fish Fry", Price: mustPrice("900.00"), IsAvailable: true},
				{Name: "Chicken Broast", Price: mustPrice("650.00"), IsAvailable: true},
			},
		},
	}
}
