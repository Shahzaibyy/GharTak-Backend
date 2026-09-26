package merchants

import (
	"context"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/domain"
	"github.com/yourusername/ghartak-backend/internal/platform/pii"
)

type store interface {
	Insert(ctx context.Context, row merchantRow) (Merchant, error)
	ListApproved(ctx context.Context, zoneID uuid.UUID) ([]Merchant, error)
	ListAvailable(ctx context.Context, merchantID uuid.UUID) ([]CatalogItem, error)
	InsertItem(ctx context.Context, merchantID uuid.UUID, item catalogRow) (CatalogItem, error)
	UpdateItem(ctx context.Context, merchantID, itemID uuid.UUID, item catalogRow) (CatalogItem, error)
	DeleteItem(ctx context.Context, merchantID, itemID uuid.UUID) error
	Pin(ctx context.Context, merchantID, zoneID uuid.UUID) (Pin, error)
	Prices(ctx context.Context, merchantID uuid.UUID, ids []uuid.UUID) ([]Price, error)
	EnsureDemo(ctx context.Context, row demoRow) (uuid.UUID, error)
	CatalogCount(ctx context.Context, merchantID uuid.UUID) (int, error)
	SetVerification(ctx context.Context, id uuid.UUID, status string) (Merchant, error)
}

var merchantDecisions = map[domain.VerificationStatus]struct{}{
	domain.VerificationApproved: {},
	domain.VerificationRejected: {},
}

type zoneChecker interface {
	Exists(ctx context.Context, id uuid.UUID) error
}

type Service struct {
	store   store
	zones   zoneChecker
	piiKey  []byte
	hashKey []byte
}

func NewService(store store, zones zoneChecker, piiKey, hashKey []byte) *Service {
	return &Service{store: store, zones: zones, piiKey: piiKey, hashKey: hashKey}
}

func (s *Service) Register(ctx context.Context, in RegisterInput) (Merchant, error) {
	if err := s.zones.Exists(ctx, in.ZoneID); err != nil {
		return Merchant{}, err
	}
	rate, err := commission(in.Category)
	if err != nil {
		return Merchant{}, err
	}
	return s.insert(ctx, in, rate)
}

func (s *Service) List(ctx context.Context, zoneID uuid.UUID) ([]Merchant, error) {
	return s.store.ListApproved(ctx, zoneID)
}

func (s *Service) Catalog(ctx context.Context, merchantID uuid.UUID) ([]CatalogItem, error) {
	return s.store.ListAvailable(ctx, merchantID)
}

func (s *Service) AddItem(ctx context.Context, merchantID uuid.UUID, in CatalogInput) (CatalogItem, error) {
	return s.store.InsertItem(ctx, merchantID, catalogFrom(in))
}

func (s *Service) UpdateItem(ctx context.Context, merchantID, itemID uuid.UUID, in CatalogInput) (CatalogItem, error) {
	return s.store.UpdateItem(ctx, merchantID, itemID, catalogFrom(in))
}

func (s *Service) DeleteItem(ctx context.Context, merchantID, itemID uuid.UUID) error {
	return s.store.DeleteItem(ctx, merchantID, itemID)
}

func (s *Service) SetVerification(ctx context.Context, id uuid.UUID, status domain.VerificationStatus) (Merchant, error) {
	if err := allowedDecision(status); err != nil {
		return Merchant{}, err
	}
	return s.store.SetVerification(ctx, id, string(status))
}

func allowedDecision(status domain.VerificationStatus) error {
	if _, ok := merchantDecisions[status]; !ok {
		return apperror.Invalid("verification status is invalid")
	}
	return nil
}

func (s *Service) ApprovedPin(ctx context.Context, merchantID, zoneID uuid.UUID) (Pin, error) {
	return s.store.Pin(ctx, merchantID, zoneID)
}

func (s *Service) Prices(ctx context.Context, merchantID uuid.UUID, ids []uuid.UUID) ([]Price, error) {
	return s.store.Prices(ctx, merchantID, ids)
}

func (s *Service) insert(ctx context.Context, in RegisterInput, rate string) (Merchant, error) {
	ciphertext, lookup, err := pii.SealPhone(s.piiKey, s.hashKey, in.Phone)
	if err != nil {
		return Merchant{}, apperror.Invalid("phone is invalid")
	}
	return s.store.Insert(ctx, merchantRow{
		Ciphertext: ciphertext, Lookup: lookup, OwnerName: in.OwnerName, Name: in.Name,
		Category: in.Category, ZoneID: in.ZoneID, Lat: in.Lat, Lng: in.Lng,
		AddressText: in.AddressText, Commission: rate,
	})
}

func commission(category Category) (string, error) {
	rate, ok := DefaultCommissionPercent(category)
	if !ok {
		return "", apperror.Invalid("category is invalid")
	}
	return rate, nil
}

func catalogFrom(in CatalogInput) catalogRow {
	return catalogRow{
		Name:        in.Name,
		Description: nullable(in.Description),
		Price:       in.Price.String(),
		IsAvailable: in.IsAvailable,
		PhotoKey:    nullable(in.PhotoKey),
	}
}

func nullable(raw string) any {
	if raw == "" {
		return nil
	}
	return raw
}
