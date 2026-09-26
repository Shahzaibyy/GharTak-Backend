package payments

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/httpx"
	"github.com/yourusername/ghartak-backend/internal/platform/money"
)

type paymentAPI interface {
	CreditWallet(ctx context.Context, in AdjustInput) error
	SettleCash(ctx context.Context, in SettleInput) error
	ApplyWebhook(ctx context.Context, ref string, status Status) error
}

type Handler struct {
	payments paymentAPI
	log      zerolog.Logger
}

func NewHandler(payments paymentAPI, log zerolog.Logger) *Handler {
	return &Handler{payments: payments, log: log}
}

type moneyBody struct {
	Amount    string `json:"amount"`
	RequestID string `json:"request_id"`
	AccountID string `json:"account_id"`
}

type webhookBody struct {
	GatewayRef string `json:"gateway_ref"`
	Status     string `json:"status"`
}

var webhookStatuses = map[string]Status{
	string(StatusHeld):     StatusHeld,
	string(StatusReleased): StatusReleased,
	string(StatusRefunded): StatusRefunded,
	string(StatusFailed):   StatusFailed,
}

func (h *Handler) CreditWallet(w http.ResponseWriter, r *http.Request) {
	in, err := decodeAdjust(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.payments.CreditWallet(r.Context(), in); err != nil {
		h.log.Error().Err(err).Msg("credit wallet")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, map[string]string{"status": "credited"})
}

func (h *Handler) SettleCash(w http.ResponseWriter, r *http.Request) {
	in, err := decodeSettle(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.payments.SettleCash(r.Context(), in); err != nil {
		h.log.Error().Err(err).Msg("settle cash")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, map[string]string{"status": "settled"})
}

func (h *Handler) Webhook(w http.ResponseWriter, r *http.Request) {
	in, status, err := decodeWebhook(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.payments.ApplyWebhook(r.Context(), in, status); err != nil {
		h.log.Error().Err(err).Str("provider", chi.URLParam(r, "provider")).Msg("payment webhook")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, map[string]string{"status": "ok"})
}

func decodeAdjust(r *http.Request) (AdjustInput, error) {
	body, amount, err := decodeMoney(r)
	if err != nil {
		return AdjustInput{}, err
	}
	accountID, err := httpx.ParseUUID(body.AccountID, "account_id")
	if err != nil {
		return AdjustInput{}, err
	}
	return AdjustInput{CustomerID: accountID, Amount: amount, RequestID: body.RequestID}, nil
}

func decodeSettle(r *http.Request) (SettleInput, error) {
	body, amount, err := decodeMoney(r)
	if err != nil {
		return SettleInput{}, err
	}
	riderID, err := httpx.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		return SettleInput{}, err
	}
	return SettleInput{RiderID: riderID, Amount: amount, RequestID: body.RequestID}, nil
}

func decodeMoney(r *http.Request) (moneyBody, money.Money, error) {
	var body moneyBody
	if err := httpx.Decode(r, &body); err != nil {
		return moneyBody{}, money.Money{}, err
	}
	if err := validRequestID(body.RequestID); err != nil {
		return moneyBody{}, money.Money{}, err
	}
	amount, err := money.Parse(body.Amount)
	if err != nil {
		return moneyBody{}, money.Money{}, apperror.Invalid("amount is invalid")
	}
	return body, amount, nil
}

func validRequestID(id string) error {
	if len(id) < 8 || len(id) > 64 {
		return apperror.Invalid("request_id is invalid")
	}
	return nil
}

func decodeWebhook(r *http.Request) (string, Status, error) {
	var body webhookBody
	if err := httpx.Decode(r, &body); err != nil {
		return "", "", err
	}
	status, ok := webhookStatuses[body.Status]
	if !ok || !validRef(body.GatewayRef) {
		return "", "", apperror.Invalid("webhook payload is invalid")
	}
	return body.GatewayRef, status, nil
}

func validRef(ref string) bool {
	return len(ref) >= 4 && len(ref) <= 100
}
