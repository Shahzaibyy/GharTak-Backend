package auth

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/httpserver"
	"github.com/yourusername/ghartak-backend/internal/platform/httpx"
)

type authenticator interface {
	RequestOTP(ctx context.Context, req OTPRequest) (OTPResult, error)
	Verify(ctx context.Context, req VerifyRequest) (Session, error)
	RequestEmailOTP(ctx context.Context, req EmailOTPRequest) (OTPResult, error)
	VerifyEmail(ctx context.Context, req EmailVerifyRequest) (Session, error)
	Refresh(ctx context.Context, refreshToken string) (Session, error)
	GoogleSignIn(ctx context.Context, idToken string) (Session, error)
	Logout(ctx context.Context, refreshToken string) error
	Profile(ctx context.Context, id uuid.UUID) (Profile, error)
	UpdateName(ctx context.Context, id uuid.UUID, name string) (Profile, error)
	SetPreferences(ctx context.Context, id uuid.UUID, types []string) (Profile, error)
	DeleteMe(ctx context.Context, id uuid.UUID) error
	PhoneLink(ctx context.Context, id uuid.UUID, phone, ip string) (OTPResult, error)
	PhoneLinkVerify(ctx context.Context, id uuid.UUID, otp string) error
}

type Handler struct {
	auth authenticator
	log  zerolog.Logger
}

func NewHandler(auth authenticator, log zerolog.Logger) *Handler {
	return &Handler{auth: auth, log: log}
}

type otpBody struct {
	Phone   string `json:"phone"`
	Role    string `json:"role"`
	Channel string `json:"channel"`
}

type verifyBody struct {
	Phone string `json:"phone"`
	Role  string `json:"role"`
	OTP   string `json:"otp"`
}

type emailBody struct {
	Email string `json:"email"`
}

type emailVerifyBody struct {
	Email string `json:"email"`
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
		Phone: body.Phone, Role: role, Channel: body.Channel,
		IP: httpserver.ClientIP(r.RemoteAddr),
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

func (h *Handler) RequestEmail(w http.ResponseWriter, r *http.Request) {
	email, err := decodeEmail(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	result, err := h.auth.RequestEmailOTP(r.Context(), EmailOTPRequest{
		Email: email, IP: httpserver.ClientIP(r.RemoteAddr),
	})
	if err != nil {
		h.log.Error().Err(err).Msg("email otp request")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	email, otp, err := decodeEmailVerify(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	session, err := h.auth.VerifyEmail(r.Context(), EmailVerifyRequest{Email: email, OTP: otp})
	if err != nil {
		h.log.Error().Err(err).Msg("email otp verify")
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

func decodeEmail(r *http.Request) (string, error) {
	var body emailBody
	if err := httpx.Decode(r, &body); err != nil {
		return "", err
	}
	return body.Email, nil
}

func decodeEmailVerify(r *http.Request) (string, string, error) {
	var body emailVerifyBody
	if err := httpx.Decode(r, &body); err != nil {
		return "", "", err
	}
	if !digits(body.OTP, 6) {
		return "", "", apperror.Invalid("otp is invalid")
	}
	return body.Email, body.OTP, nil
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
