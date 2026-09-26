package httpx_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/httpx"
)

func TestWriteError(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "invalid", err: apperror.Invalid("active must be true or false"), status: 400, code: "invalid_input", message: "active must be true or false"},
		{name: "not found", err: apperror.ErrNotFound, status: 404, code: "not_found", message: "resource not found"},
		{name: "conflict", err: apperror.ErrConflict, status: 409, code: "conflict", message: "conflict"},
		{name: "phone required", err: apperror.ConflictCode("phone_required", "verify a phone number before placing an order"), status: 409, code: "phone_required", message: "verify a phone number before placing an order"},
		{name: "rate", err: apperror.ErrRateLimited, status: 429, code: "rate_limited", message: "too many requests"},
		{name: "wrapped", err: errors.Join(errors.New("db"), apperror.ErrUnavailable), status: 503, code: "unavailable", message: "service unavailable"},
		{name: "unknown", err: errors.New("sql failed"), status: 500, code: "internal", message: "internal error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			httpx.WriteError(rec, tt.err)
			if rec.Code != tt.status {
				t.Fatalf("status = %d", rec.Code)
			}
			var body struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Error.Code != tt.code || body.Error.Message != tt.message {
				t.Fatalf("body = %+v", body.Error)
			}
		})
	}
}

func TestWriteData(t *testing.T) {
	rec := httptest.NewRecorder()
	httpx.WriteData(rec, http.StatusOK, map[string]string{"status": "ok"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Body.String() != "{\"data\":{\"status\":\"ok\"}}\n" {
		t.Fatalf("body = %s", rec.Body.String())
	}
}
