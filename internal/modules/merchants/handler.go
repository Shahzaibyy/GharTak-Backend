package merchants

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/yourusername/ghartak-backend/internal/modules/auth"
	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/domain"
	"github.com/yourusername/ghartak-backend/internal/platform/httpx"
	"github.com/yourusername/ghartak-backend/internal/platform/money"
)

type catalogAPI interface {
	Register(ctx context.Context, in RegisterInput) (Merchant, error)
	List(ctx context.Context, zoneID uuid.UUID) ([]Merchant, error)
	Catalog(ctx context.Context, merchantID uuid.UUID) ([]CatalogItem, error)
	AddItem(ctx context.Context, merchantID uuid.UUID, in CatalogInput) (CatalogItem, error)
	UpdateItem(ctx context.Context, merchantID, itemID uuid.UUID, in CatalogInput) (CatalogItem, error)
	DeleteItem(ctx context.Context, merchantID, itemID uuid.UUID) error
	SetVerification(ctx context.Context, id uuid.UUID, status domain.VerificationStatus) (Merchant, error)
}

type Handler struct {
	merchants catalogAPI
	log       zerolog.Logger
}

func NewHandler(merchants catalogAPI, log zerolog.Logger) *Handler {
	return &Handler{merchants: merchants, log: log}
}

type registerBody struct {
	Phone       string  `json:"phone"`
	OwnerName   string  `json:"owner_name"`
	Name        string  `json:"name"`
	Category    string  `json:"category"`
	ZoneID      string  `json:"zone_id"`
	Lat         float64 `json:"lat"`
	Lng         float64 `json:"lng"`
	AddressText string  `json:"address_text"`
}

type catalogBody struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Price       money.Money `json:"price"`
	IsAvailable bool        `json:"is_available"`
	PhotoKey    string      `json:"photo_key"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	in, err := decodeRegister(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	merchant, err := h.merchants.Register(r.Context(), in)
	if err != nil {
		h.log.Error().Err(err).Msg("register merchant")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusCreated, merchant)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	zoneID, err := httpx.ParseUUID(r.URL.Query().Get("zone_id"), "zone_id")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	list, err := h.merchants.List(r.Context(), zoneID)
	if err != nil {
		h.log.Error().Err(err).Msg("list merchants")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, list)
}

func (h *Handler) Catalog(w http.ResponseWriter, r *http.Request) {
	merchantID, err := httpx.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	items, err := h.merchants.Catalog(r.Context(), merchantID)
	if err != nil {
		h.log.Error().Err(err).Msg("catalog")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, items)
}

func (h *Handler) AddItem(w http.ResponseWriter, r *http.Request) {
	merchantID, item, err := decodeOwnedItem(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	created, err := h.merchants.AddItem(r.Context(), merchantID, item)
	if err != nil {
		h.log.Error().Err(err).Msg("add catalog item")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusCreated, created)
}

func (h *Handler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	merchantID, itemID, item, err := decodeOwnedUpdate(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	updated, err := h.merchants.UpdateItem(r.Context(), merchantID, itemID, item)
	if err != nil {
		h.log.Error().Err(err).Msg("update catalog item")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, updated)
}

func (h *Handler) DeleteItem(w http.ResponseWriter, r *http.Request) {
	merchantID, itemID, err := ownedIDs(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.merchants.DeleteItem(r.Context(), merchantID, itemID); err != nil {
		h.log.Error().Err(err).Msg("delete catalog item")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, map[string]bool{"deleted": true})
}

func decodeRegister(r *http.Request) (RegisterInput, error) {
	body, zoneID, category, err := readRegister(r)
	if err != nil {
		return RegisterInput{}, err
	}
	if err := validMerchantText(body); err != nil {
		return RegisterInput{}, err
	}
	return RegisterInput{
		Phone: body.Phone, OwnerName: body.OwnerName, Name: body.Name, Category: category,
		ZoneID: zoneID, Lat: body.Lat, Lng: body.Lng, AddressText: body.AddressText,
	}, validPoint(body.Lat, body.Lng)
}

func readRegister(r *http.Request) (registerBody, uuid.UUID, Category, error) {
	var body registerBody
	if err := httpx.Decode(r, &body); err != nil {
		return registerBody{}, uuid.UUID{}, "", err
	}
	zoneID, err := httpx.ParseUUID(body.ZoneID, "zone_id")
	if err != nil {
		return registerBody{}, uuid.UUID{}, "", err
	}
	category, err := validCategory(body.Category)
	return body, zoneID, category, err
}

func validCategory(raw string) (Category, error) {
	category := Category(raw)
	if _, ok := DefaultCommissionPercent(category); !ok {
		return "", apperror.Invalid("category is invalid")
	}
	return category, nil
}

func validMerchantText(body registerBody) error {
	if !textLen(body.OwnerName, 1, 100) || !textLen(body.Name, 1, 100) {
		return apperror.Invalid("name is invalid")
	}
	if !textLen(body.AddressText, 1, 500) {
		return apperror.Invalid("address_text is invalid")
	}
	return nil
}

func decodeOwnedItem(r *http.Request) (uuid.UUID, CatalogInput, error) {
	merchantID, err := ownedMerchant(r)
	if err != nil {
		return uuid.UUID{}, CatalogInput{}, err
	}
	item, err := decodeCatalog(r)
	return merchantID, item, err
}

func decodeOwnedUpdate(r *http.Request) (uuid.UUID, uuid.UUID, CatalogInput, error) {
	merchantID, itemID, err := ownedIDs(r)
	if err != nil {
		return uuid.UUID{}, uuid.UUID{}, CatalogInput{}, err
	}
	item, err := decodeCatalog(r)
	return merchantID, itemID, item, err
}

func ownedIDs(r *http.Request) (uuid.UUID, uuid.UUID, error) {
	merchantID, err := ownedMerchant(r)
	if err != nil {
		return uuid.UUID{}, uuid.UUID{}, err
	}
	itemID, err := httpx.ParseUUID(chi.URLParam(r, "item_id"), "item_id")
	return merchantID, itemID, err
}

func ownedMerchant(r *http.Request) (uuid.UUID, error) {
	merchantID, err := httpx.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		return uuid.UUID{}, err
	}
	principal, ok := auth.PrincipalFrom(r.Context())
	if !ok || principal.AccountID != merchantID {
		return uuid.UUID{}, apperror.ErrForbidden
	}
	return merchantID, nil
}

func decodeCatalog(r *http.Request) (CatalogInput, error) {
	var body catalogBody
	if err := httpx.Decode(r, &body); err != nil {
		return CatalogInput{}, err
	}
	if err := validCatalog(body); err != nil {
		return CatalogInput{}, err
	}
	return CatalogInput{
		Name: body.Name, Description: body.Description, Price: body.Price,
		IsAvailable: body.IsAvailable, PhotoKey: body.PhotoKey,
	}, nil
}

func validCatalog(body catalogBody) error {
	if !textLen(body.Name, 1, 100) || len(body.Description) > 1000 {
		return apperror.Invalid("catalog item is invalid")
	}
	return validCatalogPrice(body)
}

func validCatalogPrice(body catalogBody) error {
	if len(body.PhotoKey) > 200 || body.Price.IsNegative() {
		return apperror.Invalid("price is invalid")
	}
	return nil
}

func textLen(raw string, min, max int) bool {
	n := len(raw)
	return n >= min && n <= max
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

func (h *Handler) Approve(w http.ResponseWriter, r *http.Request) {
	h.decideMerchant(w, r, domain.VerificationApproved)
}

func (h *Handler) Reject(w http.ResponseWriter, r *http.Request) {
	h.decideMerchant(w, r, domain.VerificationRejected)
}

func (h *Handler) decideMerchant(w http.ResponseWriter, r *http.Request, status domain.VerificationStatus) {
	id, err := httpx.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	merchant, err := h.merchants.SetVerification(r.Context(), id, status)
	if err != nil {
		h.log.Error().Err(err).Str("merchant_id", id.String()).Msg("merchant verification")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, merchant)
}
