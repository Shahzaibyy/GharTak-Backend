package geo

import (
	"context"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/yourusername/ghartak-backend/internal/modules/auth"
	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	platgeo "github.com/yourusername/ghartak-backend/internal/platform/geo"
	"github.com/yourusername/ghartak-backend/internal/platform/httpx"
)

type api interface {
	Search(ctx context.Context, accountID uuid.UUID, q string, nearLat, nearLng float64, limit int, zoneID uuid.UUID) ([]Place, error)
	Reverse(ctx context.Context, accountID uuid.UUID, lat, lng float64) (Place, error)
}

type Handler struct {
	geo api
	log zerolog.Logger
}

func NewHandler(geo api, log zerolog.Logger) *Handler {
	return &Handler{geo: geo, log: log}
}

func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	accountID, q, nearLat, nearLng, limit, zoneID, err := decodeSearch(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	list, err := h.geo.Search(r.Context(), accountID, q, nearLat, nearLng, limit, zoneID)
	if err != nil {
		h.log.Error().Err(err).Msg("geo search")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, list)
}

func (h *Handler) Reverse(w http.ResponseWriter, r *http.Request) {
	accountID, lat, lng, err := decodeReverse(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	place, err := h.geo.Reverse(r.Context(), accountID, lat, lng)
	if err != nil {
		h.log.Error().Err(err).Msg("geo reverse")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, place)
}

func decodeSearch(r *http.Request) (uuid.UUID, string, float64, float64, int, uuid.UUID, error) {
	accountID, err := callerID(r)
	if err != nil {
		return uuid.UUID{}, "", 0, 0, 0, uuid.UUID{}, err
	}
	q := r.URL.Query().Get("q")
	if len(q) < 3 || len(q) > 100 {
		return uuid.UUID{}, "", 0, 0, 0, uuid.UUID{}, apperror.Invalid("q is invalid")
	}
	nearLat, nearLng, err := optionalPoint(r, "near_lat", "near_lng")
	if err != nil {
		return uuid.UUID{}, "", 0, 0, 0, uuid.UUID{}, err
	}
	limit, err := optionalLimit(r.URL.Query().Get("limit"), 5)
	if err != nil {
		return uuid.UUID{}, "", 0, 0, 0, uuid.UUID{}, err
	}
	zoneID, err := optionalUUID(r.URL.Query().Get("zone_id"))
	return accountID, q, nearLat, nearLng, limit, zoneID, err
}

func decodeReverse(r *http.Request) (uuid.UUID, float64, float64, error) {
	accountID, err := callerID(r)
	if err != nil {
		return uuid.UUID{}, 0, 0, err
	}
	lat, lng, err := requiredPoint(r, "lat", "lng")
	return accountID, lat, lng, err
}

func callerID(r *http.Request) (uuid.UUID, error) {
	principal, ok := auth.PrincipalFrom(r.Context())
	if !ok {
		return uuid.UUID{}, apperror.ErrUnauthorized
	}
	return principal.AccountID, nil
}

func requiredPoint(r *http.Request, latKey, lngKey string) (float64, float64, error) {
	lat, err := parseFloat(r.URL.Query().Get(latKey), latKey)
	if err != nil {
		return 0, 0, err
	}
	lng, err := parseFloat(r.URL.Query().Get(lngKey), lngKey)
	if err != nil {
		return 0, 0, err
	}
	if !platgeo.ValidCoord(lat, lng) {
		return 0, 0, apperror.Invalid("location is invalid")
	}
	return lat, lng, nil
}

func optionalPoint(r *http.Request, latKey, lngKey string) (float64, float64, error) {
	latRaw := r.URL.Query().Get(latKey)
	lngRaw := r.URL.Query().Get(lngKey)
	if latRaw == "" && lngRaw == "" {
		return 0, 0, nil
	}
	return requiredPoint(r, latKey, lngKey)
}

func optionalLimit(raw string, fallback int) (int, error) {
	if raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 5 {
		return 0, apperror.Invalid("limit is invalid")
	}
	return n, nil
}

func optionalUUID(raw string) (uuid.UUID, error) {
	if raw == "" {
		return uuid.UUID{}, nil
	}
	return httpx.ParseUUID(raw, "zone_id")
}

func parseFloat(raw, field string) (float64, error) {
	if raw == "" {
		return 0, apperror.Invalid(field + " is required")
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, apperror.Invalid(field + " is invalid")
	}
	return value, nil
}
