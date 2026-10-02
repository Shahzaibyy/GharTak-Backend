package riders

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/domain"
)

func TestSetOnlineRequiresApproval(t *testing.T) {
	svc := NewService(&fakeStore{err: apperror.ErrNotFound}, allowZone{}, key(), key())
	_, err := svc.SetOnline(context.Background(), uuid.New(), true)
	if !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("err = %v", err)
	}
}

func TestReviewOutcomes(t *testing.T) {
	id := uuid.New()
	zone := uuid.New()
	ready := Profile{ID: id, VerificationStatus: "approved", IsOnline: false}
	tests := []struct {
		name    string
		call    func(*Service) error
		store   *fakeStore
		wantErr error
	}{
		{
			name:  "approve pending",
			store: &fakeStore{profile: ready},
			call: func(s *Service) error {
				_, err := s.Approve(context.Background(), id, zone)
				return err
			},
		},
		{
			name:    "approve missing",
			store:   &fakeStore{err: apperror.ErrNotFound},
			wantErr: apperror.ErrConflict,
			call: func(s *Service) error {
				_, err := s.Approve(context.Background(), id, zone)
				return err
			},
		},
		{
			name:    "suspend unapproved",
			store:   &fakeStore{err: apperror.ErrNotFound},
			wantErr: apperror.ErrConflict,
			call: func(s *Service) error {
				_, err := s.Suspend(context.Background(), id)
				return err
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(tt.store, allowZone{}, key(), key())
			err := tt.call(svc)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestRegisterSealsIdentity(t *testing.T) {
	store := &fakeStore{profile: Profile{Name: "Ali"}}
	svc := NewService(store, allowZone{}, key(), hash())
	_, err := svc.Register(context.Background(), RegisterInput{
		Phone: "03001234567", Name: "Ali", CNIC: "12345-1234567-1",
		CNICObjectKey: "kyc/cnic", SelfieObjectKey: "kyc/selfie", LicenseObjectKey: "kyc/license",
		VehicleReg: "ATK-1", ZoneID: uuid.New(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if store.saved.PhoneLookup == "" || store.saved.CNICLookup == "" || store.saved.PhoneLookup == store.saved.CNICLookup {
		t.Fatal("identity was not sealed")
	}
}

func key() []byte  { return bytes32("pii-key-for-rider-service-test!") }
func hash() []byte { return bytes32("hash-key-for-rider-service-tes!") }

func bytes32(raw string) []byte {
	buf := make([]byte, 32)
	copy(buf, raw)
	return buf
}

type fakeStore struct {
	profile Profile
	err     error
	saved   sealedRider
}

func (f *fakeStore) Insert(_ context.Context, row sealedRider) (Profile, error) {
	f.saved = row
	return f.profile, f.err
}

func (f *fakeStore) InsertShell(context.Context, string, string, uuid.UUID) (Profile, error) {
	return f.profile, f.err
}

func (f *fakeStore) Get(context.Context, uuid.UUID) (Profile, error) {
	return f.profile, f.err
}

func (f *fakeStore) List(context.Context, domain.VerificationStatus) ([]Profile, error) {
	return nil, f.err
}

func (f *fakeStore) SetOnline(context.Context, uuid.UUID, bool) (Profile, error) {
	return f.profile, f.err
}

func (f *fakeStore) Approve(context.Context, uuid.UUID, uuid.UUID) (Profile, error) {
	return f.profile, f.err
}

func (f *fakeStore) Reject(context.Context, uuid.UUID) (Profile, error) {
	return f.profile, f.err
}

func (f *fakeStore) Suspend(context.Context, uuid.UUID) (Profile, error) {
	return f.profile, f.err
}

func (f *fakeStore) UpdateDetails(context.Context, uuid.UUID, detailsRow) (Profile, error) {
	return f.profile, f.err
}

func (f *fakeStore) UpdateDocuments(context.Context, uuid.UUID, DocumentsInput) (Profile, error) {
	return f.profile, f.err
}

func (f *fakeStore) Submit(context.Context, uuid.UUID) (Profile, error) {
	return f.profile, f.err
}

func (f *fakeStore) BookOrientation(context.Context, uuid.UUID, *string) (Profile, error) {
	return f.profile, f.err
}

type allowZone struct{}

func (allowZone) Exists(context.Context, uuid.UUID) error { return nil }

func (allowZone) Active(context.Context, uuid.UUID) error { return nil }
