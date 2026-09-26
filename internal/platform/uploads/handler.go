package uploads

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/yourusername/ghartak-backend/internal/platform/httpx"
)

type presigner interface {
	Presign(ctx context.Context, purpose string, ownerID uuid.UUID) (Result, error)
}

type Handler struct {
	signer presigner
	owner  func(context.Context) (uuid.UUID, error)
	log    zerolog.Logger
}

func NewHandler(signer presigner, owner func(context.Context) (uuid.UUID, error), log zerolog.Logger) *Handler {
	return &Handler{signer: signer, owner: owner, log: log}
}

type presignBody struct {
	Purpose string `json:"purpose"`
}

func (h *Handler) Presign(w http.ResponseWriter, r *http.Request) {
	ownerID, purpose, err := h.read(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	result, err := h.signer.Presign(r.Context(), purpose, ownerID)
	if err != nil {
		h.log.Error().Err(err).Str("account_id", ownerID.String()).Msg("presign")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

func (h *Handler) read(r *http.Request) (uuid.UUID, string, error) {
	ownerID, err := h.owner(r.Context())
	if err != nil {
		return uuid.UUID{}, "", err
	}
	var body presignBody
	if err := httpx.Decode(r, &body); err != nil {
		return uuid.UUID{}, "", err
	}
	return ownerID, body.Purpose, nil
}
