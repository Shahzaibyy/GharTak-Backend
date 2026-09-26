package auth

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/httpserver"
	"github.com/yourusername/ghartak-backend/internal/platform/httpx"
)

type googleBody struct {
	FirebaseIDToken string `json:"firebase_id_token"`
}

type nameBody struct {
	Name string `json:"name"`
}

type phoneBody struct {
	Phone string `json:"phone"`
}

type otpOnlyBody struct {
	OTP string `json:"otp"`
}

func (h *Handler) Google(w http.ResponseWriter, r *http.Request) {
	token, err := decodeGoogle(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	session, err := h.auth.GoogleSignIn(r.Context(), token)
	if err != nil {
		h.log.Error().Err(err).Msg("google sign-in")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, session)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var body refreshBody
	if err := httpx.Decode(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if body.RefreshToken == "" {
		httpx.WriteError(w, apperror.Invalid("refresh_token is required"))
		return
	}
	if err := h.auth.Logout(r.Context(), body.RefreshToken); err != nil {
		h.log.Error().Err(err).Msg("logout")
		httpx.WriteError(w, err)
		return
	}
	httpx.NoContent(w)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	id, err := AccountID(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	profile, err := h.auth.Profile(r.Context(), id)
	if err != nil {
		h.log.Error().Err(err).Str("account_id", id.String()).Msg("profile")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, profile)
}

func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	id, name, err := decodeName(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	profile, err := h.auth.UpdateName(r.Context(), id, name)
	if err != nil {
		h.log.Error().Err(err).Str("account_id", id.String()).Msg("update profile")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, profile)
}

func (h *Handler) DeleteMe(w http.ResponseWriter, r *http.Request) {
	id, err := AccountID(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.auth.DeleteMe(r.Context(), id); err != nil {
		h.log.Error().Err(err).Str("account_id", id.String()).Msg("delete account")
		httpx.WriteError(w, err)
		return
	}
	httpx.NoContent(w)
}

func (h *Handler) PhoneLink(w http.ResponseWriter, r *http.Request) {
	id, phone, err := decodePhoneLink(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	result, err := h.auth.PhoneLink(r.Context(), id, phone, httpserver.ClientIP(r.RemoteAddr))
	if err != nil {
		h.log.Error().Err(err).Str("account_id", id.String()).Msg("phone link")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

func (h *Handler) PhoneLinkVerify(w http.ResponseWriter, r *http.Request) {
	id, otp, err := decodePhoneVerify(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.auth.PhoneLinkVerify(r.Context(), id, otp); err != nil {
		h.log.Error().Err(err).Str("account_id", id.String()).Msg("phone link verify")
		httpx.WriteError(w, err)
		return
	}
	httpx.NoContent(w)
}

func decodeGoogle(r *http.Request) (string, error) {
	var body googleBody
	if err := httpx.Decode(r, &body); err != nil {
		return "", err
	}
	if strings.TrimSpace(body.FirebaseIDToken) == "" {
		return "", apperror.Invalid("firebase_id_token is required")
	}
	return body.FirebaseIDToken, nil
}

func decodeName(r *http.Request) (uuid.UUID, string, error) {
	id, err := AccountID(r.Context())
	if err != nil {
		return uuid.UUID{}, "", err
	}
	name, err := readName(r)
	return id, name, err
}

func readName(r *http.Request) (string, error) {
	var body nameBody
	if err := httpx.Decode(r, &body); err != nil {
		return "", err
	}
	name := strings.TrimSpace(body.Name)
	if len(name) < 1 || len(name) > 100 {
		return "", apperror.Invalid("name is invalid")
	}
	return name, nil
}

func decodePhoneLink(r *http.Request) (uuid.UUID, string, error) {
	id, err := AccountID(r.Context())
	if err != nil {
		return uuid.UUID{}, "", err
	}
	var body phoneBody
	if err := httpx.Decode(r, &body); err != nil {
		return uuid.UUID{}, "", err
	}
	return id, body.Phone, nil
}

func decodePhoneVerify(r *http.Request) (uuid.UUID, string, error) {
	id, err := AccountID(r.Context())
	if err != nil {
		return uuid.UUID{}, "", err
	}
	var body otpOnlyBody
	if err := httpx.Decode(r, &body); err != nil {
		return uuid.UUID{}, "", err
	}
	if !digits(body.OTP, 6) {
		return uuid.UUID{}, "", apperror.Invalid("otp is invalid")
	}
	return id, body.OTP, nil
}
