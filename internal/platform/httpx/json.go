package httpx

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
)

type dataEnvelope struct {
	Data any `json:"data"`
}

type errorEnvelope struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type mappedError struct {
	sentinel error
	status   int
	code     string
	message  string
}

var catalog = []mappedError{
	{apperror.ErrInvalidInput, http.StatusBadRequest, "invalid_input", "request is invalid"},
	{apperror.ErrUnauthorized, http.StatusUnauthorized, "unauthorized", "authentication required"},
	{apperror.ErrForbidden, http.StatusForbidden, "forbidden", "not allowed"},
	{apperror.ErrNotFound, http.StatusNotFound, "not_found", "resource not found"},
	{apperror.ErrConflict, http.StatusConflict, "conflict", "conflict"},
	{apperror.ErrRateLimited, http.StatusTooManyRequests, "rate_limited", "too many requests"},
	{apperror.ErrUnavailable, http.StatusServiceUnavailable, "unavailable", "service unavailable"},
}

func Decode(r *http.Request, dst any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return apperror.Invalid("request body is invalid")
	}
	return nil
}

func ParseUUID(raw, field string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.UUID{}, apperror.Invalid(field + " is invalid")
	}
	return id, nil
}

func WriteData(w http.ResponseWriter, status int, data any) {
	write(w, status, dataEnvelope{Data: data})
}

func WriteError(w http.ResponseWriter, err error) {
	status, code, message := classify(err)
	write(w, status, errorEnvelope{Error: apiError{Code: code, Message: message}})
}

func WriteInternal(w http.ResponseWriter) {
	write(w, http.StatusInternalServerError, errorEnvelope{
		Error: apiError{Code: "internal", Message: "internal error"},
	})
}

func classify(err error) (int, string, string) {
	for _, item := range catalog {
		if errors.Is(err, item.sentinel) {
			return item.status, item.code, publicMessage(err, item.message)
		}
	}
	return http.StatusInternalServerError, "internal", "internal error"
}

func publicMessage(err error, fallback string) string {
	var pub *apperror.Error
	if errors.As(err, &pub) && pub.Message() != "" {
		return pub.Message()
	}
	return fallback
}

func write(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
