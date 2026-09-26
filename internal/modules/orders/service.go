package orders

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/modules/admin"
	"github.com/yourusername/ghartak-backend/internal/modules/merchants"
	"github.com/yourusername/ghartak-backend/internal/modules/notifications"
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
	GetByID(ctx context.Context, orderID uuid.UUID) (Order, error)
	Party(ctx context.Context, orderID uuid.UUID) (Party, error)
	ListByMerchant(ctx context.Context, merchantID uuid.UUID, status Status) ([]Order, error)
	ListOffers(ctx context.Context, riderID uuid.UUID) ([]Order, error)
	ListTasks(ctx context.Context, riderID uuid.UUID) ([]Order, error)
	Advance(ctx context.Context, in advance) (Order, error)
	AcceptOffer(ctx context.Context, riderID, orderID uuid.UUID) (Order, error)
	RejectOffer(ctx context.Context, riderID, orderID uuid.UUID) (Order, error)
	Deliver(ctx context.Context, in deliverInput) (Order, error)
	Cancel(ctx context.Context, customerID, orderID uuid.UUID, reason string) (Order, error)
}

type statusNotifier interface {
	OrderStatus(ctx context.Context, notice notifications.Notice)
}

type offerer interface {
	Offer(ctx context.Context, orderID uuid.UUID) error
}

type phoneGate interface {
	RequirePhone(ctx context.Context, customerID uuid.UUID) error
}

type Service struct {
	zones    zoneSource
	catalog  catalogSource
	orders   orderStore
	otpKey   []byte
	notifier statusNotifier
	offers   offerer
	phones   phoneGate
}

func NewService(zones zoneSource, catalog catalogSource, orders orderStore) *Service {
	return &Service{zones: zones, catalog: catalog, orders: orders}
}

func (s *Service) WithFlow(otpKey []byte, notify statusNotifier, offers offerer) *Service {
	s.otpKey = otpKey
	s.notifier = notify
	s.offers = offers
	return s
}

func (s *Service) UsePhoneGate(gate phoneGate) *Service {
	s.phones = gate
	return s
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
	if err := s.gatePhone(ctx, customerID); err != nil {
		return Order{}, false, err
	}
	row, err := s.build(ctx, customerID, in)
	if err != nil {
		return Order{}, false, err
	}
	row, err = s.sealDelivery(row)
	if err != nil {
		return Order{}, false, err
	}
	return s.placed(ctx, row)
}

func (s *Service) gatePhone(ctx context.Context, customerID uuid.UUID) error {
	if s.phones == nil {
		return nil
	}
	return s.phones.RequirePhone(ctx, customerID)
}

func (s *Service) placed(ctx context.Context, row draft) (Order, bool, error) {
	order, created, err := s.orders.Insert(ctx, row)
	if err != nil {
		return Order{}, false, err
	}
	if !created {
		return order, false, nil
	}
	order.DeliveryOTP = row.DeliveryOTP
	return s.finishPlace(ctx, order)
}

func (s *Service) finishPlace(ctx context.Context, order Order) (Order, bool, error) {
	s.notify(ctx, order)
	if RequiresMerchant(order.Type) {
		return order, true, nil
	}
	fresh, err := s.reload(ctx, order)
	return fresh, true, err
}

func (s *Service) sealDelivery(row draft) (draft, error) {
	if !RequiresMerchant(row.Input.Type) || len(s.otpKey) == 0 {
		return row, nil
	}
	otp, hash, err := deliveryCode(s.otpKey)
	if err != nil {
		return draft{}, err
	}
	row.DeliveryOTP = otp
	row.DeliveryHash = hash
	return row, nil
}

func (s *Service) notify(ctx context.Context, order Order) {
	if s.notifier == nil {
		return
	}
	s.notifier.OrderStatus(ctx, notifications.Notice{
		OrderID: order.ID, Status: string(order.Status), CustomerID: order.CustomerID,
		MerchantID: order.MerchantID, RiderID: order.RiderID,
	})
}

func (s *Service) offer(ctx context.Context, orderID uuid.UUID) error {
	if s.offers == nil {
		return nil
	}
	return s.offers.Offer(ctx, orderID)
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
