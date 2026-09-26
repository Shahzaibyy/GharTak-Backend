package support

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

type supportAPI interface {
	Rate(ctx context.Context, in RatingInput) error
	Open(ctx context.Context, customerID, orderID uuid.UUID, body string) (Ticket, error)
	List(ctx context.Context, customerID uuid.UUID) ([]Ticket, error)
	Get(ctx context.Context, customerID, ticketID uuid.UUID) (Ticket, error)
	Messages(ctx context.Context, customerID, ticketID uuid.UUID) ([]Message, error)
	Reply(ctx context.Context, customerID, ticketID uuid.UUID, role string, senderID uuid.UUID, body string) (Message, error)
}

type Handler struct {
	support supportAPI
	log     zerolog.Logger
}

func NewHandler(support supportAPI, log zerolog.Logger) *Handler {
	return &Handler{support: support, log: log}
}

type ratingBody struct {
	RateeRole string `json:"ratee_role"`
	RateeID   string `json:"ratee_id"`
	Score     int    `json:"score"`
	Comment   string `json:"comment"`
}

type textBody struct {
	OrderID string `json:"order_id"`
	Body    string `json:"body"`
}

func (h *Handler) Rate(w http.ResponseWriter, r *http.Request) {
	in, err := decodeRating(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.support.Rate(r.Context(), in); err != nil {
		h.log.Error().Err(err).Str("order_id", in.OrderID.String()).Msg("rate order")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusCreated, map[string]string{"status": "saved"})
}

func (h *Handler) Open(w http.ResponseWriter, r *http.Request) {
	customerID, orderID, body, err := decodeTicket(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	ticket, err := h.support.Open(r.Context(), customerID, orderID, body)
	if err != nil {
		h.log.Error().Err(err).Msg("open ticket")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusCreated, ticket)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	principal, err := principalOf(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	tickets, err := h.support.List(r.Context(), principal.AccountID)
	if err != nil {
		h.log.Error().Err(err).Msg("list tickets")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, tickets)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	customerID, ticketID, err := ticketIDs(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	ticket, err := h.support.Get(r.Context(), customerID, ticketID)
	if err != nil {
		h.log.Error().Err(err).Msg("get ticket")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, ticket)
}

func (h *Handler) Messages(w http.ResponseWriter, r *http.Request) {
	customerID, ticketID, err := ticketIDs(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	messages, err := h.support.Messages(r.Context(), customerID, ticketID)
	if err != nil {
		h.log.Error().Err(err).Msg("ticket messages")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, messages)
}

func (h *Handler) Reply(w http.ResponseWriter, r *http.Request) {
	customerID, ticketID, body, err := decodeReply(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	msg, err := h.support.Reply(r.Context(), customerID, ticketID, string(auth.RoleCustomer), customerID, body)
	if err != nil {
		h.log.Error().Err(err).Msg("ticket reply")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusCreated, msg)
}

func decodeRating(r *http.Request) (RatingInput, error) {
	principal, err := principalOf(r)
	if err != nil {
		return RatingInput{}, err
	}
	orderID, err := httpx.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		return RatingInput{}, err
	}
	return ratingFrom(r, principal, orderID)
}

func ratingFrom(r *http.Request, principal auth.Principal, orderID uuid.UUID) (RatingInput, error) {
	var body ratingBody
	if err := httpx.Decode(r, &body); err != nil {
		return RatingInput{}, err
	}
	rateeID, err := httpx.ParseUUID(body.RateeID, "ratee_id")
	if err != nil {
		return RatingInput{}, err
	}
	return RatingInput{
		OrderID: orderID, RaterRole: string(principal.Role), RaterID: principal.AccountID,
		RateeRole: body.RateeRole, RateeID: rateeID, Score: body.Score, Comment: body.Comment,
	}, nil
}

func decodeTicket(r *http.Request) (uuid.UUID, uuid.UUID, string, error) {
	principal, err := principalOf(r)
	if err != nil {
		return uuid.UUID{}, uuid.UUID{}, "", err
	}
	var body textBody
	if err := httpx.Decode(r, &body); err != nil {
		return uuid.UUID{}, uuid.UUID{}, "", err
	}
	orderID, err := httpx.ParseUUID(body.OrderID, "order_id")
	if err != nil {
		return uuid.UUID{}, uuid.UUID{}, "", err
	}
	return principal.AccountID, orderID, body.Body, nil
}

func decodeReply(r *http.Request) (uuid.UUID, uuid.UUID, string, error) {
	customerID, ticketID, err := ticketIDs(r)
	if err != nil {
		return uuid.UUID{}, uuid.UUID{}, "", err
	}
	var body textBody
	if err := httpx.Decode(r, &body); err != nil {
		return uuid.UUID{}, uuid.UUID{}, "", err
	}
	return customerID, ticketID, body.Body, nil
}

func ticketIDs(r *http.Request) (uuid.UUID, uuid.UUID, error) {
	principal, err := principalOf(r)
	if err != nil {
		return uuid.UUID{}, uuid.UUID{}, err
	}
	ticketID, err := httpx.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		return uuid.UUID{}, uuid.UUID{}, err
	}
	return principal.AccountID, ticketID, nil
}

func principalOf(r *http.Request) (auth.Principal, error) {
	principal, ok := auth.PrincipalFrom(r.Context())
	if !ok {
		return auth.Principal{}, apperror.ErrUnauthorized
	}
	return principal, nil
}
