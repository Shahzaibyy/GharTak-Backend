package riders

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/domain"
	"github.com/yourusername/ghartak-backend/internal/platform/pii"
)

type store interface {
	Insert(ctx context.Context, row sealedRider) (Profile, error)
	Get(ctx context.Context, id uuid.UUID) (Profile, error)
	List(ctx context.Context, status domain.VerificationStatus) ([]Profile, error)
	SetOnline(ctx context.Context, id uuid.UUID, online bool) (Profile, error)
	Approve(ctx context.Context, id, zoneID uuid.UUID) (Profile, error)
	Reject(ctx context.Context, id uuid.UUID) (Profile, error)
	Suspend(ctx context.Context, id uuid.UUID) (Profile, error)
}

type zoneGate interface {
	Exists(ctx context.Context, id uuid.UUID) error
	Active(ctx context.Context, id uuid.UUID) error
}

type Service struct {
	store    store
	zones    zoneGate
	piiKey   []byte
	hashKey  []byte
	presence presence
}

func NewService(store store, zones zoneGate, piiKey, hashKey []byte) *Service {
	return &Service{store: store, zones: zones, piiKey: piiKey, hashKey: hashKey}
}

func (s *Service) Register(ctx context.Context, in RegisterInput) (Profile, error) {
	if err := s.zones.Exists(ctx, in.ZoneID); err != nil {
		return Profile{}, err
	}
	row, err := s.seal(in)
	if err != nil {
		return Profile{}, err
	}
	return s.store.Insert(ctx, row)
}

func (s *Service) Me(ctx context.Context, id uuid.UUID) (Profile, error) {
	return s.store.Get(ctx, id)
}

func (s *Service) SetOnline(ctx context.Context, id uuid.UUID, online bool) (Profile, error) {
	profile, err := s.store.SetOnline(ctx, id, online)
	if errors.Is(err, apperror.ErrNotFound) && online {
		return Profile{}, apperror.Forbidden("rider must be approved and assigned to a zone")
	}
	if err != nil {
		return profile, err
	}
	s.syncPresence(ctx, profile)
	return profile, nil
}

func (s *Service) List(ctx context.Context, status domain.VerificationStatus) ([]Profile, error) {
	return s.store.List(ctx, status)
}

func (s *Service) Approve(ctx context.Context, id, zoneID uuid.UUID) (Profile, error) {
	if err := s.zones.Active(ctx, zoneID); err != nil {
		return Profile{}, err
	}
	profile, err := s.store.Approve(ctx, id, zoneID)
	return s.finish(profile, err, "rider is not awaiting verification")
}

func (s *Service) Reject(ctx context.Context, id uuid.UUID) (Profile, error) {
	profile, err := s.store.Reject(ctx, id)
	return s.finish(profile, err, "rider is not awaiting verification")
}

func (s *Service) Suspend(ctx context.Context, id uuid.UUID) (Profile, error) {
	profile, err := s.store.Suspend(ctx, id)
	return s.finish(profile, err, "rider is not approved")
}

func (s *Service) finish(profile Profile, err error, message string) (Profile, error) {
	if errors.Is(err, apperror.ErrNotFound) {
		return Profile{}, apperror.Conflict(message)
	}
	return profile, err
}

func (s *Service) seal(in RegisterInput) (sealedRider, error) {
	phone, phoneLookup, err := pii.SealPhone(s.piiKey, s.hashKey, in.Phone)
	if err != nil {
		return sealedRider{}, apperror.Invalid("phone is invalid")
	}
	cnic, cnicLookup, err := pii.SealCNIC(s.piiKey, s.hashKey, in.CNIC)
	if err != nil {
		return sealedRider{}, apperror.Invalid("cnic is invalid")
	}
	return sealedRider{
		PhoneCipher: phone, PhoneLookup: phoneLookup, Name: in.Name,
		CNICCipher: cnic, CNICLookup: cnicLookup,
		CNICObjectKey: in.CNICObjectKey, SelfieObjectKey: in.SelfieObjectKey,
		LicenseObjectKey: in.LicenseObjectKey, VehicleReg: in.VehicleReg, ZoneID: in.ZoneID,
	}, nil
}
