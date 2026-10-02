package customers

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
)

func TestCompleteOnboardingNormalizesLabel(t *testing.T) {
	store := &memAddresses{}
	profiles := &memProfiles{}
	svc := NewService(store)
	svc.UseProfiles(profiles)
	result, err := svc.CompleteOnboarding(context.Background(), uuid.New(), OnboardingInput{
		Name: "Ayesha",
		Address: AddressInput{
			Label: "Home", Lat: 33.7, Lng: 72.3, AddressText: "Attock Road",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Address.Label != "home" || profiles.name != "Ayesha" {
		t.Fatalf("result = %+v name = %s", result, profiles.name)
	}
}

func TestCompleteOnboardingRejectsBadLabel(t *testing.T) {
	svc := NewService(&memAddresses{})
	svc.UseProfiles(&memProfiles{})
	_, err := svc.CompleteOnboarding(context.Background(), uuid.New(), OnboardingInput{
		Name: "Ayesha",
		Address: AddressInput{Label: "school", Lat: 33.7, Lng: 72.3, AddressText: "x"},
	})
	if err == nil || err.Error() != apperror.Invalid("label is invalid").Error() {
		t.Fatalf("err = %v", err)
	}
}

type memAddresses struct {
	last AddressInput
}

func (m *memAddresses) Insert(_ context.Context, userID uuid.UUID, in AddressInput) (Address, error) {
	m.last = in
	return Address{ID: uuid.New(), UserID: userID, Label: in.Label, Lat: in.Lat, Lng: in.Lng, AddressText: in.AddressText}, nil
}

func (m *memAddresses) List(context.Context, uuid.UUID) ([]Address, error) { return nil, nil }

func (m *memAddresses) Update(context.Context, uuid.UUID, uuid.UUID, AddressInput) (Address, error) {
	return Address{}, nil
}

func (m *memAddresses) Delete(context.Context, uuid.UUID, uuid.UUID) error { return nil }

type memProfiles struct{ name string }

func (m *memProfiles) UpdateNameOnly(_ context.Context, _ uuid.UUID, name string) error {
	m.name = name
	return nil
}
