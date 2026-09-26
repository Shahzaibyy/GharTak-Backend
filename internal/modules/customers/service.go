package customers

import (
	"context"

	"github.com/google/uuid"
)

type store interface {
	Insert(ctx context.Context, userID uuid.UUID, in AddressInput) (Address, error)
	List(ctx context.Context, userID uuid.UUID) ([]Address, error)
	Update(ctx context.Context, userID, addressID uuid.UUID, in AddressInput) (Address, error)
	Delete(ctx context.Context, userID, addressID uuid.UUID) error
}

type Service struct {
	store store
}

func NewService(store store) *Service {
	return &Service{store: store}
}

func (s *Service) Create(ctx context.Context, userID uuid.UUID, in AddressInput) (Address, error) {
	return s.store.Insert(ctx, userID, in)
}

func (s *Service) List(ctx context.Context, userID uuid.UUID) ([]Address, error) {
	return s.store.List(ctx, userID)
}

func (s *Service) Update(ctx context.Context, userID, addressID uuid.UUID, in AddressInput) (Address, error) {
	return s.store.Update(ctx, userID, addressID, in)
}

func (s *Service) Delete(ctx context.Context, userID, addressID uuid.UUID) error {
	return s.store.Delete(ctx, userID, addressID)
}
