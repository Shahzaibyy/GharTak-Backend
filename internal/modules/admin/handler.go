package admin

import (
	"context"
	"net/http"
	"strconv"

	"github.com/rs/zerolog"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/httpx"
)

type ZoneLister interface {
	ListZones(ctx context.Context, activeOnly bool) ([]Zone, error)
}

type Handler struct {
	zones ZoneLister
	log   zerolog.Logger
}

func NewHandler(zones ZoneLister, log zerolog.Logger) *Handler {
	return &Handler{zones: zones, log: log}
}

func (h *Handler) ListZones(w http.ResponseWriter, r *http.Request) {
	activeOnly, err := parseActiveOnly(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	zones, err := h.zones.ListZones(r.Context(), activeOnly)
	if err != nil {
		h.log.Error().Err(err).Msg("list zones")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, zones)
}

func parseActiveOnly(r *http.Request) (bool, error) {
	raw := r.URL.Query().Get("active")
	if raw == "" {
		return false, nil
	}
	active, err := strconv.ParseBool(raw)
	if err != nil {
		return false, apperror.Invalid("active must be true or false")
	}
	return active, nil
}
