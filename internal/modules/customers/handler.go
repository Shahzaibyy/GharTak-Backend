package customers

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/yourusername/ghartak-backend/internal/modules/auth"
	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/httpx"
)

type addressAPI interface {
	Create(ctx context.Context, userID uuid.UUID, in AddressInput) (Address, error)
	List(ctx context.Context, userID uuid.UUID) ([]Address, error)
	Update(ctx context.Context, userID, addressID uuid.UUID, in AddressInput) (Address, error)
	Delete(ctx context.Context, userID, addressID uuid.UUID) error
}

type Handler struct {
	addresses addressAPI
	log       zerolog.Logger
}

func NewHandler(addresses addressAPI, log zerolog.Logger) *Handler {
	return &Handler{addresses: addresses, log: log}
}

type addressBody struct {
	Label       string  `json:"label"`
	Lat         float64 `json:"lat"`
	Lng         float64 `json:"lng"`
	AddressText string  `json:"address_text"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID, in, err := decodeOwnAddress(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	address, err := h.addresses.Create(r.Context(), userID, in)
	if err != nil {
		h.log.Error().Err(err).Msg("create address")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusCreated, address)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID, err := callerID(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	list, err := h.addresses.List(r.Context(), userID)
	if err != nil {
		h.log.Error().Err(err).Msg("list addresses")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, list)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	userID, addressID, in, err := decodeOwnUpdate(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	address, err := h.addresses.Update(r.Context(), userID, addressID, in)
	if err != nil {
		h.log.Error().Err(err).Msg("update address")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, address)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, addressID, err := ownAddressID(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.addresses.Delete(r.Context(), userID, addressID); err != nil {
		h.log.Error().Err(err).Msg("delete address")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, map[string]bool{"deleted": true})
}

func decodeOwnAddress(r *http.Request) (uuid.UUID, AddressInput, error) {
	userID, err := callerID(r)
	if err != nil {
		return uuid.UUID{}, AddressInput{}, err
	}
	in, err := decodeAddress(r)
	return userID, in, err
}

func decodeOwnUpdate(r *http.Request) (uuid.UUID, uuid.UUID, AddressInput, error) {
	userID, addressID, err := ownAddressID(r)
	if err != nil {
		return uuid.UUID{}, uuid.UUID{}, AddressInput{}, err
	}
	in, err := decodeAddress(r)
	return userID, addressID, in, err
}

func ownAddressID(r *http.Request) (uuid.UUID, uuid.UUID, error) {
	userID, err := callerID(r)
	if err != nil {
		return uuid.UUID{}, uuid.UUID{}, err
	}
	addressID, err := httpx.ParseUUID(chi.URLParam(r, "id"), "id")
	return userID, addressID, err
}

func callerID(r *http.Request) (uuid.UUID, error) {
	principal, ok := auth.PrincipalFrom(r.Context())
	if !ok {
		return uuid.UUID{}, apperror.ErrUnauthorized
	}
	return principal.AccountID, nil
}

func decodeAddress(r *http.Request) (AddressInput, error) {
	var body addressBody
	if err := httpx.Decode(r, &body); err != nil {
		return AddressInput{}, err
	}
	if err := validAddress(body); err != nil {
		return AddressInput{}, err
	}
	return AddressInput{Label: body.Label, Lat: body.Lat, Lng: body.Lng, AddressText: body.AddressText}, nil
}

func validAddress(body addressBody) error {
	if !between(len(body.Label), 1, 50) || !between(len(body.AddressText), 1, 500) {
		return apperror.Invalid("address is invalid")
	}
	return validPoint(body.Lat, body.Lng)
}

func between(n, min, max int) bool {
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
