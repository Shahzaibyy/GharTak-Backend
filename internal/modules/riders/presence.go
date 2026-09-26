package riders

import (
	"context"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
)

type presence interface {
	Online(ctx context.Context, zoneID, riderID uuid.UUID, lat, lng float64) error
	Offline(ctx context.Context, zoneID, riderID uuid.UUID) error
}

func (s *Service) UsePresence(p presence) {
	s.presence = p
}

func (s *Service) Position(ctx context.Context, id uuid.UUID, lat, lng float64) error {
	profile, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := readyPosition(profile); err != nil {
		return err
	}
	return s.markOnline(ctx, profile, lat, lng)
}

func readyPosition(profile Profile) error {
	if !profile.IsOnline || profile.ZoneID == nil {
		return apperror.Forbidden("rider must be online in a zone")
	}
	return nil
}

func (s *Service) markOnline(ctx context.Context, profile Profile, lat, lng float64) error {
	if s.presence == nil {
		return nil
	}
	return s.presence.Online(ctx, *profile.ZoneID, profile.ID, lat, lng)
}

func (s *Service) syncPresence(ctx context.Context, profile Profile) {
	if s.presence == nil || profile.ZoneID == nil || profile.IsOnline {
		return
	}
	_ = s.presence.Offline(ctx, *profile.ZoneID, profile.ID)
}
