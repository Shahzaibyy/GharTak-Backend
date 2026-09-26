package orders

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
)

func TestPlaceBlocksUnverifiedPhone(t *testing.T) {
	store := &gateStore{}
	svc := NewService(nil, nil, store).UsePhoneGate(denyPhone{})
	_, _, err := svc.Place(context.Background(), uuid.New(), PlaceInput{ClientRequestID: "req-1"})
	if !errors.Is(err, apperror.ErrConflict) {
		t.Fatalf("err = %v", err)
	}
	var coded *apperror.Error
	if !errors.As(err, &coded) || coded.Code() != "phone_required" || store.inserted {
		t.Fatalf("err = %v inserted = %v", err, store.inserted)
	}
}

type denyPhone struct{}

func (denyPhone) RequirePhone(context.Context, uuid.UUID) error {
	return apperror.ConflictCode("phone_required", "verify a phone number before placing an order")
}

type gateStore struct {
	inserted bool
}

func (s *gateStore) FindByClientRequest(context.Context, uuid.UUID, string) (Order, error) {
	return Order{}, apperror.ErrNotFound
}

func (s *gateStore) Insert(context.Context, draft) (Order, bool, error) {
	s.inserted = true
	return Order{}, false, nil
}

func (s *gateStore) Get(context.Context, uuid.UUID, uuid.UUID) (Order, error) {
	return Order{}, apperror.ErrNotFound
}

func (s *gateStore) GetByID(context.Context, uuid.UUID) (Order, error) {
	return Order{}, apperror.ErrNotFound
}

func (s *gateStore) Party(context.Context, uuid.UUID) (Party, error) {
	return Party{}, apperror.ErrNotFound
}

func (s *gateStore) ListByMerchant(context.Context, uuid.UUID, Status) ([]Order, error) {
	return nil, nil
}

func (s *gateStore) ListOffers(context.Context, uuid.UUID) ([]Order, error) { return nil, nil }

func (s *gateStore) ListTasks(context.Context, uuid.UUID) ([]Order, error) { return nil, nil }

func (s *gateStore) Advance(context.Context, advance) (Order, error) {
	return Order{}, apperror.ErrNotFound
}

func (s *gateStore) AcceptOffer(context.Context, uuid.UUID, uuid.UUID) (Order, error) {
	return Order{}, apperror.ErrNotFound
}

func (s *gateStore) RejectOffer(context.Context, uuid.UUID, uuid.UUID) (Order, error) {
	return Order{}, apperror.ErrNotFound
}

func (s *gateStore) Deliver(context.Context, deliverInput) (Order, error) {
	return Order{}, apperror.ErrNotFound
}

func (s *gateStore) Cancel(context.Context, uuid.UUID, uuid.UUID, string) (Order, error) {
	return Order{}, apperror.ErrNotFound
}
