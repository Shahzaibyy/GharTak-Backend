package auth

import (
	"context"

	"github.com/google/uuid"
)

// UpdateNameOnly adapts UpdateName for customer onboarding without returning the full profile.
func (s *Service) UpdateNameOnly(ctx context.Context, id uuid.UUID, name string) error {
	_, err := s.UpdateName(ctx, id, name)
	return err
}
