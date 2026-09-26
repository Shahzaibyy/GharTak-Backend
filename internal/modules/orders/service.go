package orders

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/modules/admin"
	"github.com/yourusername/ghartak-backend/internal/modules/merchants"
	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/money"
)

type zoneSource interface {
	ActiveZone(ctx context.Context, id uuid.UUID) (admin.Zone, error)
}

type catalogSource interface {
	ApprovedPin(ctx context.Context, merchantID, zoneID uuid.UUID) (merchants.Pin, error)
	Prices(ctx context.Context, merchantID uuid.UUID, ids []uuid.UUID) ([]merchants.Price, error)
}

type orderStore interface {
	FindByClientRequest(ctx context.Context, customerID uuid.UUID, requestID string) (Order, error)
	Insert(ctx context.Context, row draft) (Order, bool, error)
	Get(ctx context.Context, customerID, orderID uuid.UUID) (Order, error)
}

type Service struct {
	zones   zoneSource
	catalog catalogSource
	orders  orderStore
}

func NewService(zones zoneSource, catalog catalogSource, orders orderStore) *Service {
	return &Service{zones: zones, catalog: catalog, orders: orders}
}

func (s *Service) Quote(ctx context.Context, in PlaceInput) (Priced, error) {
	row, err := s.build(ctx, uuid.Nil, in)
	if err != nil {
		return Priced{}, err
	}
	return row.Price, nil
}

func (s *Service) Place(ctx context.Context, customerID uuid.UUID, in PlaceInput) (Order, bool, error) {
	existing, err := s.orders.FindByClientRequest(ctx, customerID, in.ClientRequestID)
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, apperror.ErrNotFound) {
		return Order{}, false, err
	}
	return s.create(ctx, customerID, in)
}

func (s *Service) Get(ctx context.Context, customerID, orderID uuid.UUID) (Order, error) {
	return s.orders.Get(ctx, customerID, orderID)
}

func (s *Service) create(ctx context.Context, customerID uuid.UUID, in PlaceInput) (Order, bool, error) {
	row, err := s.build(ctx, customerID, in)
	if err != nil {
		return Order{}, false, err
	}
	return s.orders.Insert(ctx, row)
}

func (s *Service) build(ctx context.Context, customerID uuid.UUID, in PlaceInput) (draft, error) {
	zone, err := s.zones.ActiveZone(ctx, in.ZoneID)
	if err != nil {
		return draft{}, err
	}
	return s.dispatch(ctx, customerID, in, zone)
}

func (s *Service) dispatch(ctx context.Context, customerID uuid.UUID, in PlaceInput, zone admin.Zone) (draft, error) {
	if RequiresMerchant(in.Type) {
		return s.buildCatalog(ctx, customerID, in, zone)
	}
	return quoteDraft(customerID, in, zone, nil, money.FromPaisa(0), nil, "0.00")
}

func (s *Service) buildCatalog(ctx context.Context, customerID uuid.UUID, in PlaceInput, zone admin.Zone) (draft, error) {
	pin, err := s.catalog.ApprovedPin(ctx, *in.MerchantID, in.ZoneID)
	if errors.Is(err, apperror.ErrNotFound) {
		return draft{}, apperror.Invalid("merchant is not approved in this zone")
	}
	if err != nil {
		return draft{}, err
	}
	return s.priceCatalog(ctx, customerID, in, zone, pin)
}

func (s *Service) priceCatalog(ctx context.Context, customerID uuid.UUID, in PlaceInput, zone admin.Zone, pin merchants.Pin) (draft, error) {
	prices, err := s.catalog.Prices(ctx, pin.ID, itemIDs(in.Items))
	if err != nil {
		return draft{}, err
	}
	lines, stored, err := matchLines(prices, in.Items)
	if err != nil {
		return draft{}, err
	}
	total, err := ItemTotal(lines)
	if err != nil {
		return draft{}, err
	}
	return quoteDraft(customerID, in, zone, &pin, total, stored, pin.CommissionRate)
}

func quoteDraft(customerID uuid.UUID, in PlaceInput, zone admin.Zone, pin *merchants.Pin, itemTotal money.Money, items []OrderItem, commission string) (draft, error) {
	lat, lng, address := pickupOf(in, pin)
	priced, err := Price(cardOf(zone), distanceMeters(lat, lng, in.DropLat, in.DropLng), in.Effort, itemTotal, commission)
	if err != nil {
		return draft{}, priceErr(err)
	}
	return draft{
		CustomerID: customerID, Input: in, PickupLat: lat, PickupLng: lng,
		PickupAddress: address, Items: items, Price: priced,
	}, nil
}

func pickupOf(in PlaceInput, pin *merchants.Pin) (float64, float64, string) {
	if pin == nil {
		return in.PickupLat, in.PickupLng, in.PickupAddress
	}
	return pin.Lat, pin.Lng, pin.AddressText
}

func cardOf(zone admin.Zone) RateCard {
	return RateCard{Base: zone.BaseDeliveryFee, PerKm: zone.PerKmRate, Surge: zone.SurgeMultiplier}
}

func matchLines(prices []merchants.Price, items []ItemInput) ([]Line, []OrderItem, error) {
	byID := indexPrices(prices)
	if len(byID) != len(items) {
		return nil, nil, apperror.Invalid("catalog item is unavailable")
	}
	return pairLines(byID, items)
}

func pairLines(byID map[uuid.UUID]merchants.Price, items []ItemInput) ([]Line, []OrderItem, error) {
	lines := make([]Line, 0, len(items))
	stored := make([]OrderItem, 0, len(items))
	for _, item := range items {
		price, ok := byID[item.CatalogItemID]
		if !ok {
			return nil, nil, apperror.Invalid("catalog item is unavailable")
		}
		lines, stored = appendLine(lines, stored, item, price)
	}
	return lines, stored, nil
}

func appendLine(lines []Line, stored []OrderItem, item ItemInput, price merchants.Price) ([]Line, []OrderItem) {
	id := price.ID
	lines = append(lines, Line{Price: price.Price, Qty: int64(item.Quantity)})
	stored = append(stored, OrderItem{CatalogItemID: &id, Name: price.Name, Quantity: item.Quantity, Price: price.Price})
	return lines, stored
}

func indexPrices(prices []merchants.Price) map[uuid.UUID]merchants.Price {
	byID := make(map[uuid.UUID]merchants.Price, len(prices))
	for _, price := range prices {
		byID[price.ID] = price
	}
	return byID
}

func itemIDs(items []ItemInput) []uuid.UUID {
	ids := make([]uuid.UUID, len(items))
	for i, item := range items {
		ids[i] = item.CatalogItemID
	}
	return ids
}

func priceErr(err error) error {
	if errors.Is(err, money.ErrInvalid) || errors.Is(err, ErrUnknownEffort) {
		return apperror.Invalid("price is invalid")
	}
	return err
}
