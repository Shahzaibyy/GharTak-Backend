package customers

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/domain"
)

type store interface {
	Insert(ctx context.Context, userID uuid.UUID, in AddressInput) (Address, error)
	List(ctx context.Context, userID uuid.UUID) ([]Address, error)
	Update(ctx context.Context, userID, addressID uuid.UUID, in AddressInput) (Address, error)
	Delete(ctx context.Context, userID, addressID uuid.UUID) error
}

type profileWriter interface {
	UpdateName(ctx context.Context, id uuid.UUID, name string) error
}

type Service struct {
	store    store
	profiles profileWriter
}

func NewService(store store) *Service {
	return &Service{store: store}
}

func (s *Service) UseProfiles(profiles profileWriter) {
	s.profiles = profiles
}

type OnboardingInput struct {
	Name    string
	Address AddressInput
}

type OnboardingResult struct {
	Name    string  `json:"name"`
	Address Address `json:"address"`
}

func (s *Service) Create(ctx context.Context, userID uuid.UUID, in AddressInput) (Address, error) {
	if err := normalizeLabel(&in); err != nil {
		return Address{}, err
	}
	return s.store.Insert(ctx, userID, in)
}

func (s *Service) List(ctx context.Context, userID uuid.UUID) ([]Address, error) {
	return s.store.List(ctx, userID)
}

func (s *Service) Update(ctx context.Context, userID, addressID uuid.UUID, in AddressInput) (Address, error) {
	if err := normalizeLabel(&in); err != nil {
		return Address{}, err
	}
	return s.store.Update(ctx, userID, addressID, in)
}

func (s *Service) Delete(ctx context.Context, userID, addressID uuid.UUID) error {
	return s.store.Delete(ctx, userID, addressID)
}

func (s *Service) CompleteOnboarding(ctx context.Context, userID uuid.UUID, in OnboardingInput) (OnboardingResult, error) {
	name := strings.TrimSpace(in.Name)
	if len(name) < 1 || len(name) > 100 {
		return OnboardingResult{}, apperror.Invalid("name is invalid")
	}
	if err := normalizeLabel(&in.Address); err != nil {
		return OnboardingResult{}, err
	}
	if s.profiles == nil {
		return OnboardingResult{}, apperror.ErrUnavailable
	}
	if err := s.profiles.UpdateName(ctx, userID, name); err != nil {
		return OnboardingResult{}, err
	}
	address, err := s.store.Insert(ctx, userID, in.Address)
	if err != nil {
		return OnboardingResult{}, err
	}
	return OnboardingResult{Name: name, Address: address}, nil
}

func normalizeLabel(in *AddressInput) error {
	label := strings.ToLower(strings.TrimSpace(in.Label))
	if _, ok := addressLabels[domain.AddressLabel(label)]; !ok {
		return apperror.Invalid("label is invalid")
	}
	in.Label = label
	return nil
}

var addressLabels = map[domain.AddressLabel]struct{}{
	domain.AddressHome:  {},
	domain.AddressWork:  {},
	domain.AddressOther: {},
}
