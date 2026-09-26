package auth

import (
	"context"
	"net/http"

	"github.com/rs/zerolog"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/httpserver"
	"github.com/yourusername/ghartak-backend/internal/platform/httpx"
)

type authenticator interface {
	RequestOTP(ctx context.Context, req OTPRequest) (OTPResult, error)
	Verify(ctx context.Context, req VerifyRequest) (Session, error)
	Refresh(ctx context.Context, refreshToken string) (Session, error)
}

type Handler struct {
	auth authenticator
	log  zerolog.Logger
}

func NewHandler(auth authenticator, log zerolog.Logger) *Handler {
	return &Handler{auth: auth, log: log}
}

type otpBody struct {
	Phone string `json:"phone"`
	Role  string `json:"role"`
}

type verifyBody struct {
	Phone string `json:"phone"`
	Role  string `json:"role"`
	OTP   string `json:"otp"`
}

type refreshBody struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) RequestOTP(w http.ResponseWriter, r *http.Request) {
	body, role, err := decodeOTP(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	result, err := h.auth.RequestOTP(r.Context(), OTPRequest{
		Phone: body.Phone,
		Role:  role,
		IP:    httpserver.ClientIP(r.RemoteAddr),
	})
	if err != nil {
		h.log.Error().Err(err).Str("role", string(role)).Msg("otp request")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

func (h *Handler) Verify(w http.ResponseWriter, r *http.Request) {
	body, role, err := decodeVerify(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	session, err := h.auth.Verify(r.Context(), VerifyRequest{Phone: body.Phone, Role: role, OTP: body.OTP})
	if err != nil {
		h.log.Error().Err(err).Str("role", string(role)).Msg("otp verify")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, session)
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var body refreshBody
	if err := httpx.Decode(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if body.RefreshToken == "" {
		httpx.WriteError(w, apperror.Invalid("refresh_token is required"))
		return
	}
	session, err := h.auth.Refresh(r.Context(), body.RefreshToken)
	if err != nil {
		h.log.Error().Err(err).Msg("refresh")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, session)
}

func decodeOTP(r *http.Request) (otpBody, Role, error) {
	var body otpBody
	if err := httpx.Decode(r, &body); err != nil {
		return otpBody{}, "", err
	}
	role, err := requiredRole(body.Role)
	return body, role, err
}

func decodeVerify(r *http.Request) (verifyBody, Role, error) {
	var body verifyBody
	if err := httpx.Decode(r, &body); err != nil {
		return verifyBody{}, "", err
	}
	role, err := requiredRole(body.Role)
	if err != nil {
		return verifyBody{}, "", err
	}
	if !digits(body.OTP, 6) {
		return verifyBody{}, "", apperror.Invalid("otp is invalid")
	}
	return body, role, nil
}

func requiredRole(raw string) (Role, error) {
	role, ok := ParseRole(raw)
	if !ok {
		return "", apperror.Invalid("role is invalid")
	}
	return role, nil
}

func digits(raw string, n int) bool {
	if len(raw) != n {
		return false
	}
	return allDigits(raw)
}

func allDigits(raw string) bool {
	for _, r := range raw {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
