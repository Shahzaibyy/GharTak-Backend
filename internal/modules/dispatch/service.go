package dispatch

import (
	"context"
	"log"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/modules/orders"
	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
)

// ErrNoRiders is returned when dispatch cannot place an offer.
var ErrNoRiders = apperror.Unavailable("no online rider available in this zone")

type store interface {
	View(ctx context.Context, orderID uuid.UUID) (View, error)
	Offered(ctx context.Context, orderID uuid.UUID) (map[uuid.UUID]struct{}, error)
	Expire(ctx context.Context, orderID uuid.UUID) error
	SaveOffer(ctx context.Context, view View, riderID uuid.UUID) error
	OnlineInZone(ctx context.Context, zoneID uuid.UUID, skip map[uuid.UUID]struct{}) (uuid.UUID, error)
}

type nearest interface {
	Nearest(ctx context.Context, zoneID uuid.UUID, lng, lat float64, km int, skip map[uuid.UUID]struct{}) (uuid.UUID, error)
}

type offerTimer interface {
	ScheduleOfferTimeout(ctx context.Context, orderID uuid.UUID) error
}

type Service struct {
	store    store
	geo      nearest
	timeouts offerTimer
}

func NewService(store store, geo nearest) *Service {
	return &Service{store: store, geo: geo}
}

func (s *Service) UseOfferTimer(timer offerTimer) *Service {
	s.timeouts = timer
	return s
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
		return ErrNoRiders
	}
	if err := s.store.SaveOffer(ctx, view, rider); err != nil {
		return err
	}
	s.scheduleTimeout(ctx, view.ID)
	return nil
}

func (s *Service) scheduleTimeout(ctx context.Context, orderID uuid.UUID) {
	if s.timeouts == nil {
		return
	}
	if err := s.timeouts.ScheduleOfferTimeout(ctx, orderID); err != nil {
		log.Printf("dispatch: schedule offer timeout for %s: %v", orderID, err)
	}
}

func (s *Service) pick(ctx context.Context, view View) (uuid.UUID, error) {
	offered, err := s.store.Offered(ctx, view.ID)
	if err != nil {
		return uuid.Nil, err
	}
	rider, err := s.firstFree(ctx, view, offered)
	if err != nil {
		// Redis geo failures must not 500 dispatch — fall back to DB online riders.
		log.Printf("dispatch: geo nearest failed for order %s: %v", view.ID, err)
		return s.store.OnlineInZone(ctx, view.ZoneID, offered)
	}
	if rider != uuid.Nil {
		return rider, nil
	}
	return s.store.OnlineInZone(ctx, view.ZoneID, offered)
}

func (s *Service) firstFree(ctx context.Context, view View, offered map[uuid.UUID]struct{}) (uuid.UUID, error) {
	steps := Steps(view.RadiusKm)
	if len(steps) == 0 {
		steps = []int{1, 3, 5}
	}
	for _, km := range steps {
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
