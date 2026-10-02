package orders

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/modules/auth"
	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/httpx"
)

func (h *Handler) MerchantOrders(w http.ResponseWriter, r *http.Request) {
	merchantID, err := callerID(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	status, err := parseStatus(r.URL.Query().Get("status"), StatusPlaced)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	orders, err := h.orders.ListMerchant(r.Context(), merchantID, status)
	if err != nil {
		h.log.Error().Err(err).Msg("list merchant orders")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, orders)
}

func (h *Handler) MerchantAction(w http.ResponseWriter, r *http.Request) {
	merchantID, orderID, err := ownOrderID(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	order, err := h.orders.MerchantAction(r.Context(), merchantID, orderID, chi.URLParam(r, "action"))
	if err != nil {
		h.log.Error().Err(err).Msg("merchant order action")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, order)
}

func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	customerID, orderID, err := ownOrderID(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	reason, err := cancelReason(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	order, err := h.orders.Cancel(r.Context(), customerID, orderID, reason)
	if err != nil {
		h.log.Error().Err(err).Msg("cancel order")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, order)
}

func (h *Handler) Offers(w http.ResponseWriter, r *http.Request) {
	h.listRider(w, r, func(id uuid.UUID) ([]Order, error) {
		return h.orders.ListOffers(r.Context(), id)
	})
}

func (h *Handler) Tasks(w http.ResponseWriter, r *http.Request) {
	h.listRider(w, r, func(id uuid.UUID) ([]Order, error) {
		return h.orders.ListTasks(r.Context(), id)
	})
}

func (h *Handler) Accept(w http.ResponseWriter, r *http.Request) {
	h.riderOrder(w, r, func(riderID, orderID uuid.UUID) (Order, error) {
		return h.orders.Accept(r.Context(), riderID, orderID)
	})
}

func (h *Handler) Reject(w http.ResponseWriter, r *http.Request) {
	h.riderOrder(w, r, func(riderID, orderID uuid.UUID) (Order, error) {
		return h.orders.Reject(r.Context(), riderID, orderID)
	})
}

func (h *Handler) Pickup(w http.ResponseWriter, r *http.Request) {
	h.riderAct(w, r, "pickup")
}

func (h *Handler) Enroute(w http.ResponseWriter, r *http.Request) {
	h.riderAct(w, r, "enroute")
}

func (h *Handler) riderAct(w http.ResponseWriter, r *http.Request, action string) {
	h.riderOrder(w, r, func(riderID, orderID uuid.UUID) (Order, error) {
		return h.orders.RiderAction(r.Context(), riderID, orderID, action)
	})
}

func (h *Handler) Deliver(w http.ResponseWriter, r *http.Request) {
	h.riderOrder(w, r, func(riderID, orderID uuid.UUID) (Order, error) {
		otp, proof, err := deliverBody(r)
		if err != nil {
			return Order{}, err
		}
		return h.orders.Deliver(r.Context(), riderID, orderID, otp, proof)
	})
}

func (h *Handler) Dispatch(w http.ResponseWriter, r *http.Request) {
	principal, orderID, err := principalOrder(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	order, err := h.orders.DispatchFor(r.Context(), string(principal.Role), principal.AccountID, orderID)
	if err != nil {
		h.log.Error().Err(err).Msg("dispatch order")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, order)
}

func (h *Handler) listRider(w http.ResponseWriter, r *http.Request, list func(uuid.UUID) ([]Order, error)) {
	riderID, err := callerID(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	orders, err := list(riderID)
	if err != nil {
		h.log.Error().Err(err).Msg("list rider orders")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, orders)
}

func (h *Handler) riderOrder(w http.ResponseWriter, r *http.Request, run func(uuid.UUID, uuid.UUID) (Order, error)) {
	riderID, orderID, err := ownOrderID(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	order, err := run(riderID, orderID)
	if err != nil {
		h.log.Error().Err(err).Msg("rider task")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, order)
}

type cancelBody struct {
	Reason string `json:"reason"`
}

func cancelReason(r *http.Request) (string, error) {
	var body cancelBody
	if err := httpx.Decode(r, &body); err != nil {
		return "", err
	}
	return body.Reason, nil
}

func deliverBody(r *http.Request) (string, string, error) {
	raw, err := readBody(r, 1<<20)
	if err != nil {
		return "", "", err
	}
	if len(raw) == 0 {
		return "", "", apperror.Invalid("body required: {\"delivery_otp\":\"1234\"} for food/mart, or {\"proof_photo_key\":\"...\"} for courier/errand")
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		return "", "", apperror.Invalid("request body must be JSON")
	}
	otp := firstString(fields, "delivery_otp", "otp")
	proof := firstString(fields, "proof_photo_key", "proof")
	if otp == "" && proof == "" {
		return "", "", apperror.Invalid("delivery_otp (food/mart) or proof_photo_key (courier/errand) is required")
	}
	return otp, proof, nil
}

func readBody(r *http.Request, limit int64) ([]byte, error) {
	defer r.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if err != nil {
		return nil, apperror.Invalid("request body is invalid")
	}
	if int64(len(raw)) > limit {
		return nil, apperror.Invalid("request body is too large")
	}
	return bytes.TrimSpace(raw), nil
}

func firstString(fields map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := fields[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			return strings.TrimSpace(typed)
		case float64:
			return strings.TrimSpace(strconv.FormatInt(int64(typed), 10))
		case json.Number:
			return strings.TrimSpace(typed.String())
		}
	}
	return ""
}

func parseStatus(raw string, fallback Status) (Status, error) {
	if raw == "" {
		return fallback, nil
	}
	for _, status := range Statuses() {
		if string(status) == raw {
			return status, nil
		}
	}
	return "", apperror.Invalid("status is invalid")
}

func principalOrder(r *http.Request) (auth.Principal, uuid.UUID, error) {
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
