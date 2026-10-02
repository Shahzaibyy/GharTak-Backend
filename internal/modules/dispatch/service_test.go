package dispatch

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/modules/orders"
	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
)

func TestOfferCreatesForNearestRider(t *testing.T) {
	orderID := uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa1")
	riderID := uuid.MustParse("33333333-3333-4333-8333-333333333301")
	zoneID := uuid.MustParse("11111111-1111-4111-8111-111111111104")
	store := &fakeStore{view: View{
		ID: orderID, Type: orders.TypeFood, Status: orders.StatusReadyForPickup,
		ZoneID: zoneID, PickupLat: 33.5676, PickupLng: 72.6419, RadiusKm: 8,
	}}
	geo := &fakeGeo{rider: riderID}
	svc := NewService(store, geo)
	if err := svc.Offer(context.Background(), orderID); err != nil {
		t.Fatal(err)
	}
	if store.saved != riderID {
		t.Fatalf("saved rider = %s", store.saved)
	}
}

func TestOfferFallsBackWhenGeoErrors(t *testing.T) {
	orderID := uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa2")
	riderID := uuid.MustParse("33333333-3333-4333-8333-333333333301")
	zoneID := uuid.MustParse("11111111-1111-4111-8111-111111111104")
	store := &fakeStore{
		view: View{
			ID: orderID, Type: orders.TypeFood, Status: orders.StatusReadyForPickup,
			ZoneID: zoneID, PickupLat: 33.5676, PickupLng: 72.6419, RadiusKm: 8,
		},
		online: riderID,
	}
	svc := NewService(store, &fakeGeo{err: errors.New("ERR unknown command 'GEOSEARCH'")})
	if err := svc.Offer(context.Background(), orderID); err != nil {
		t.Fatal(err)
	}
	if store.saved != riderID {
		t.Fatalf("saved rider = %s", store.saved)
	}
}

func TestOfferNoRidersUnavailable(t *testing.T) {
	orderID := uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa3")
	store := &fakeStore{view: View{
		ID: orderID, Type: orders.TypeFood, Status: orders.StatusReadyForPickup,
		ZoneID: uuid.New(), RadiusKm: 8,
	}}
	svc := NewService(store, &fakeGeo{})
	err := svc.Offer(context.Background(), orderID)
	if !errors.Is(err, apperror.ErrUnavailable) {
		t.Fatalf("err = %v", err)
	}
}

func TestOfferRejectsWrongStatus(t *testing.T) {
	store := &fakeStore{view: View{
		ID: uuid.New(), Type: orders.TypeFood, Status: orders.StatusPreparing, RadiusKm: 8,
	}}
	err := NewService(store, &fakeGeo{}).Offer(context.Background(), store.view.ID)
	if !errors.Is(err, apperror.ErrConflict) {
		t.Fatalf("err = %v", err)
	}
}

func TestRadiusKm(t *testing.T) {
	km, err := radiusKm("8.00")
	if err != nil || km != 8 {
		t.Fatalf("km=%d err=%v", km, err)
	}
}

type fakeStore struct {
	view   View
	saved  uuid.UUID
	online uuid.UUID
}

func (f *fakeStore) View(context.Context, uuid.UUID) (View, error) { return f.view, nil }
func (f *fakeStore) Offered(context.Context, uuid.UUID) (map[uuid.UUID]struct{}, error) {
	return map[uuid.UUID]struct{}{}, nil
}
func (f *fakeStore) Expire(context.Context, uuid.UUID) error { return nil }
func (f *fakeStore) SaveOffer(_ context.Context, _ View, riderID uuid.UUID) error {
	f.saved = riderID
	return nil
}
func (f *fakeStore) OnlineInZone(context.Context, uuid.UUID, map[uuid.UUID]struct{}) (uuid.UUID, error) {
	return f.online, nil
}

type fakeGeo struct {
	rider uuid.UUID
	err   error
}

func (f *fakeGeo) Nearest(context.Context, uuid.UUID, float64, float64, int, map[uuid.UUID]struct{}) (uuid.UUID, error) {
	return f.rider, f.err
}
