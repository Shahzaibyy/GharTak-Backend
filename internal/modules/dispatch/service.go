package dispatch

import (
	"context"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/modules/orders"
	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
)

type store interface {
	View(ctx context.Context, orderID uuid.UUID) (View, error)
	Offered(ctx context.Context, orderID uuid.UUID) (map[uuid.UUID]struct{}, error)
	Expire(ctx context.Context, orderID uuid.UUID) error
	SaveOffer(ctx context.Context, view View, riderID uuid.UUID) error
}

type nearest interface {
	Nearest(ctx context.Context, zoneID uuid.UUID, lng, lat float64, km int, skip map[uuid.UUID]struct{}) (uuid.UUID, error)
}

type Service struct {
	store store
	geo   nearest
}

func NewService(store store, geo nearest) *Service {
	return &Service{store: store, geo: geo}
}

func (s *Service) Offer(ctx context.Context, orderID uuid.UUID) error {
	view, err := s.store.View(ctx, orderID)
	if err != nil {
		return err
	}
	if err := s.store.Expire(ctx, orderID); err != nil {
		return err
	}
	return s.offerNearest(ctx, view)
}

func (s *Service) offerNearest(ctx context.Context, view View) error {
	if err := canOffer(view); err != nil {
		return err
	}
	rider, err := s.pick(ctx, view)
	if err != nil {
		return err
	}
	if rider == uuid.Nil {
		return nil
	}
	return s.store.SaveOffer(ctx, view, rider)
}

func (s *Service) pick(ctx context.Context, view View) (uuid.UUID, error) {
	offered, err := s.store.Offered(ctx, view.ID)
	if err != nil {
		return uuid.Nil, err
	}
	return s.firstFree(ctx, view, offered)
}

func (s *Service) firstFree(ctx context.Context, view View, offered map[uuid.UUID]struct{}) (uuid.UUID, error) {
	for _, km := range Steps(view.RadiusKm) {
		rider, err := s.geo.Nearest(ctx, view.ZoneID, view.PickupLng, view.PickupLat, km, offered)
		if err != nil {
			return uuid.Nil, err
		}
		if rider != uuid.Nil {
			return rider, nil
		}
	}
	return uuid.Nil, nil
}

func canOffer(view View) error {
	if view.Status == orders.StatusRiderOffered {
		return nil
	}
	if err := orders.Transition(view.Type, view.Status, orders.StatusRiderOffered); err != nil {
		return apperror.Conflict("order is not ready to offer")
	}
	return nil
}
