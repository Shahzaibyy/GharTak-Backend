package chat

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

type chatAPI interface {
	Send(ctx context.Context, role string, senderID, orderID uuid.UUID, body string) (Message, error)
	List(ctx context.Context, role string, senderID, orderID uuid.UUID) ([]Message, error)
}

type Handler struct {
	chat chatAPI
	log  zerolog.Logger
}

func NewHandler(chat chatAPI, log zerolog.Logger) *Handler {
	return &Handler{chat: chat, log: log}
}

type bodyPayload struct {
	Body string `json:"body"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	principal, orderID, err := chatIDs(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	messages, err := h.chat.List(r.Context(), string(principal.Role), principal.AccountID, orderID)
	if err != nil {
		h.log.Error().Err(err).Str("order_id", orderID.String()).Msg("list chat")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, messages)
}

func (h *Handler) Send(w http.ResponseWriter, r *http.Request) {
	principal, orderID, body, err := decodeSend(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	msg, err := h.chat.Send(r.Context(), string(principal.Role), principal.AccountID, orderID, body)
	if err != nil {
		h.log.Error().Err(err).Str("order_id", orderID.String()).Msg("send chat")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusCreated, msg)
}

func decodeSend(r *http.Request) (auth.Principal, uuid.UUID, string, error) {
	principal, orderID, err := chatIDs(r)
	if err != nil {
		return auth.Principal{}, uuid.UUID{}, "", err
	}
	var body bodyPayload
	if err := httpx.Decode(r, &body); err != nil {
		return auth.Principal{}, uuid.UUID{}, "", err
	}
	return principal, orderID, body.Body, nil
}

func chatIDs(r *http.Request) (auth.Principal, uuid.UUID, error) {
	principal, ok := auth.PrincipalFrom(r.Context())
	if !ok {
		return auth.Principal{}, uuid.UUID{}, apperror.ErrUnauthorized
	}
	orderID, err := httpx.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		return auth.Principal{}, uuid.UUID{}, err
	}
	return principal, orderID, nil
}
