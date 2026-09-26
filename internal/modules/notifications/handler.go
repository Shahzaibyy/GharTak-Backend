package notifications

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/yourusername/ghartak-backend/internal/modules/auth"
	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/httpx"
)

type tokenSaver interface {
	Save(ctx context.Context, accountID uuid.UUID, role, token string) error
}

type Handler struct {
	tokens tokenSaver
	log    zerolog.Logger
}

func NewHandler(tokens tokenSaver, log zerolog.Logger) *Handler {
	return &Handler{tokens: tokens, log: log}
}

type tokenBody struct {
	Token string `json:"token"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	principal, token, err := decodeToken(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.tokens.Save(r.Context(), principal.AccountID, string(principal.Role), token); err != nil {
		h.log.Error().Err(err).Str("account_id", principal.AccountID.String()).Msg("save device token")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, map[string]string{"status": "saved"})
}

func decodeToken(r *http.Request) (auth.Principal, string, error) {
	principal, ok := auth.PrincipalFrom(r.Context())
	if !ok {
		return auth.Principal{}, "", apperror.ErrUnauthorized
	}
	var body tokenBody
	if err := httpx.Decode(r, &body); err != nil {
		return auth.Principal{}, "", err
	}
	if err := validToken(body.Token); err != nil {
		return auth.Principal{}, "", err
	}
	return principal, body.Token, nil
}

func validToken(token string) error {
	if len(token) < 10 || len(token) > 512 {
		return apperror.Invalid("token is invalid")
	}
	return nil
}
