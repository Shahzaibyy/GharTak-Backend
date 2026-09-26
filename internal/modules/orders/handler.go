package orders

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/yourusername/ghartak-backend/internal/modules/auth"
	"github.com/yourusername/ghartak-backend/internal/modules/payments"
	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/httpx"
)

type orderAPI interface {
	Quote(ctx context.Context, in PlaceInput) (Priced, error)
	Place(ctx context.Context, customerID uuid.UUID, in PlaceInput) (Order, bool, error)
	Get(ctx context.Context, customerID, orderID uuid.UUID) (Order, error)
}

type Handler struct {
	orders orderAPI
	log    zerolog.Logger
}

func NewHandler(orders orderAPI, log zerolog.Logger) *Handler {
	return &Handler{orders: orders, log: log}
}

func (h *Handler) Quote(w http.ResponseWriter, r *http.Request) {
	_, in, err := decodePlace(r, false)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	priced, err := h.orders.Quote(r.Context(), in)
	if err != nil {
		h.log.Error().Err(err).Msg("quote order")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, priced)
}

func (h *Handler) Place(w http.ResponseWriter, r *http.Request) {
	customerID, in, err := decodePlace(r, true)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	order, created, err := h.orders.Place(r.Context(), customerID, in)
	if err != nil {
		h.log.Error().Err(err).Msg("place order")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, placeStatus(created), order)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	customerID, orderID, err := ownOrderID(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	order, err := h.orders.Get(r.Context(), customerID, orderID)
	if err != nil {
		h.log.Error().Err(err).Msg("get order")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, order)
}

func placeStatus(created bool) int {
	if created {
		return http.StatusCreated
	}
	return http.StatusOK
}

type placeBody struct {
	Type            string     `json:"type"`
	ZoneID          string     `json:"zone_id"`
	MerchantID      *string    `json:"merchant_id"`
	PickupLat       float64    `json:"pickup_lat"`
	PickupLng       float64    `json:"pickup_lng"`
	PickupAddress   string     `json:"pickup_address"`
	DropLat         float64    `json:"drop_lat"`
	DropLng         float64    `json:"drop_lng"`
	DropAddress     string     `json:"drop_address"`
	Description     string     `json:"description"`
	EffortTier      *string    `json:"effort_tier"`
	Items           []itemBody `json:"items"`
	PaymentMethod   string     `json:"payment_method"`
	ClientRequestID string     `json:"client_request_id"`
}

type itemBody struct {
	CatalogItemID string `json:"catalog_item_id"`
	Quantity      int    `json:"quantity"`
}

type common struct {
	ZoneID          uuid.UUID
	PaymentMethod   string
	ClientRequestID string
}

type checker func(Type, placeBody, common) (PlaceInput, error)

var kindCheck = map[Type]checker{
	TypeFood:    checkCatalog,
	TypeMart:    checkCatalog,
	TypeCourier: checkDirect,
	TypeErrand:  checkDirect,
}

var methodSet = map[payments.Method]struct{}{
	payments.MethodJazzCash:  {},
	payments.MethodEasyPaisa: {},
	payments.MethodCOD:       {},
	payments.MethodWallet:    {},
}

func decodePlace(r *http.Request, requireRequest bool) (uuid.UUID, PlaceInput, error) {
	customerID, err := callerID(r)
	if err != nil {
		return uuid.UUID{}, PlaceInput{}, err
	}
	in, err := validateBody(r, requireRequest)
	return customerID, in, err
}

func validateBody(r *http.Request, requireRequest bool) (PlaceInput, error) {
	var body placeBody
	if err := httpx.Decode(r, &body); err != nil {
		return PlaceInput{}, err
	}
	kind, err := parseType(body.Type)
	if err != nil {
		return PlaceInput{}, err
	}
	base, err := commonFields(body, requireRequest)
	if err != nil {
		return PlaceInput{}, err
	}
	return kindCheck[kind](kind, body, base)
}

func commonFields(body placeBody, requireRequest bool) (common, error) {
	zoneID, method, err := zoneAndMethod(body)
	if err != nil {
		return common{}, err
	}
	if err := validDrop(body); err != nil {
		return common{}, err
	}
	return finishCommon(body, zoneID, method, requireRequest)
}

func finishCommon(body placeBody, zoneID uuid.UUID, method string, requireRequest bool) (common, error) {
	base := common{ZoneID: zoneID, PaymentMethod: method}
	if !requireRequest {
		return base, nil
	}
	if err := validRequestID(body.ClientRequestID); err != nil {
		return common{}, err
	}
	base.ClientRequestID = body.ClientRequestID
	return base, nil
}

func zoneAndMethod(body placeBody) (uuid.UUID, string, error) {
	zoneID, err := httpx.ParseUUID(body.ZoneID, "zone_id")
	if err != nil {
		return uuid.UUID{}, "", err
	}
	if _, ok := methodSet[payments.Method(body.PaymentMethod)]; !ok {
		return uuid.UUID{}, "", apperror.Invalid("payment_method is invalid")
	}
	return zoneID, body.PaymentMethod, nil
}

func parseType(raw string) (Type, error) {
	kind := Type(raw)
	if _, ok := kindCheck[kind]; !ok {
		return "", apperror.Invalid("type is invalid")
	}
	return kind, nil
}

func checkCatalog(kind Type, body placeBody, base common) (PlaceInput, error) {
	merchantID, err := requiredID(body.MerchantID)
	if err != nil {
		return PlaceInput{}, err
	}
	items, err := validItems(body.Items)
	if err != nil {
		return PlaceInput{}, err
	}
	in := base.input(kind, body)
	in.MerchantID = &merchantID
	in.Items = items
	return in, nil
}

func checkDirect(kind Type, body placeBody, base common) (PlaceInput, error) {
	if err := rejectCatalog(body); err != nil {
		return PlaceInput{}, err
	}
	if err := validPickup(body); err != nil {
		return PlaceInput{}, err
	}
	return finishDirect(kind, body, base)
}

func finishDirect(kind Type, body placeBody, base common) (PlaceInput, error) {
	if err := validText(body.Description, 1, 2000, "description is invalid"); err != nil {
		return PlaceInput{}, err
	}
	effort, err := validEffort(body.EffortTier)
	if err != nil {
		return PlaceInput{}, err
	}
	in := base.input(kind, body)
	in.Effort = effort
	return in, nil
}

func (base common) input(kind Type, body placeBody) PlaceInput {
	return PlaceInput{
		Type: kind, ZoneID: base.ZoneID, PaymentMethod: base.PaymentMethod, ClientRequestID: base.ClientRequestID,
		PickupLat: body.PickupLat, PickupLng: body.PickupLng, PickupAddress: body.PickupAddress,
		DropLat: body.DropLat, DropLng: body.DropLng, DropAddress: body.DropAddress, Description: body.Description,
	}
}

func rejectCatalog(body placeBody) error {
	if body.MerchantID != nil || len(body.Items) > 0 {
		return apperror.Invalid("merchant_id and items are only for food and mart")
	}
	return nil
}

func requiredID(raw *string) (uuid.UUID, error) {
	if raw == nil {
		return uuid.UUID{}, apperror.Invalid("merchant_id is required")
	}
	return httpx.ParseUUID(*raw, "merchant_id")
}

func validItems(items []itemBody) ([]ItemInput, error) {
	if len(items) < 1 || len(items) > 30 {
		return nil, apperror.Invalid("items must contain 1 to 30 entries")
	}
	return mapItems(items)
}

func mapItems(items []itemBody) ([]ItemInput, error) {
	out := make([]ItemInput, 0, len(items))
	for _, item := range items {
		parsed, err := oneItem(item)
		if err != nil {
			return nil, err
		}
		out = append(out, parsed)
	}
	return out, nil
}

func oneItem(item itemBody) (ItemInput, error) {
	id, err := httpx.ParseUUID(item.CatalogItemID, "catalog_item_id")
	if err != nil {
		return ItemInput{}, err
	}
	if item.Quantity < 1 || item.Quantity > 20 {
		return ItemInput{}, apperror.Invalid("quantity must be 1 to 20")
	}
	return ItemInput{CatalogItemID: id, Quantity: item.Quantity}, nil
}

func validPickup(body placeBody) error {
	if err := validPoint(body.PickupLat, body.PickupLng); err != nil {
		return err
	}
	return validText(body.PickupAddress, 1, 500, "pickup_address is invalid")
}

func validDrop(body placeBody) error {
	if err := validPoint(body.DropLat, body.DropLng); err != nil {
		return err
	}
	return validText(body.DropAddress, 1, 500, "drop_address is invalid")
}

func validEffort(raw *string) (*EffortTier, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	tier := EffortTier(*raw)
	if _, err := EffortBPS(tier); err != nil {
		return nil, apperror.Invalid("effort_tier is invalid")
	}
	return &tier, nil
}

func validRequestID(id string) error {
	if len(id) < 8 || len(id) > 64 {
		return apperror.Invalid("client_request_id must be 8 to 64 characters")
	}
	return nil
}

func validText(raw string, min, max int, message string) error {
	if len(raw) < min || len(raw) > max {
		return apperror.Invalid(message)
	}
	return nil
}

func validPoint(lat, lng float64) error {
	if lat < -90 || lat > 90 {
		return apperror.Invalid("location is invalid")
	}
	return validLng(lng)
}

func validLng(lng float64) error {
	if lng < -180 || lng > 180 {
		return apperror.Invalid("location is invalid")
	}
	return nil
}

func ownOrderID(r *http.Request) (uuid.UUID, uuid.UUID, error) {
	customerID, err := callerID(r)
	if err != nil {
		return uuid.UUID{}, uuid.UUID{}, err
	}
	orderID, err := httpx.ParseUUID(chi.URLParam(r, "id"), "id")
	return customerID, orderID, err
}

func callerID(r *http.Request) (uuid.UUID, error) {
	principal, ok := auth.PrincipalFrom(r.Context())
	if !ok {
		return uuid.UUID{}, apperror.ErrUnauthorized
	}
	return principal.AccountID, nil
}
