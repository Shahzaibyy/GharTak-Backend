package admin

import (
	"context"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
)

type ZoneReader interface {
	List(ctx context.Context, activeOnly bool) ([]Zone, error)
	Get(ctx context.Context, id uuid.UUID) (Zone, error)
}

type Service struct {
	zones ZoneReader
}

func NewService(zones ZoneReader) *Service {
	return &Service{zones: zones}
}

func (s *Service) ListZones(ctx context.Context, activeOnly bool) ([]Zone, error) {
	return s.zones.List(ctx, activeOnly)
}

func (s *Service) Exists(ctx context.Context, id uuid.UUID) error {
	_, err := s.zones.Get(ctx, id)
	return err
}

func (s *Service) ActiveZone(ctx context.Context, id uuid.UUID) (Zone, error) {
	zone, err := s.zones.Get(ctx, id)
	if err != nil {
		return Zone{}, err
	}
	if !zone.IsActive {
		return Zone{}, apperror.Invalid("zone is not active")
	}
	return zone, nil
}
