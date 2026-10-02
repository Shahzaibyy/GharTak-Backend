package geo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/yourusername/ghartak-backend/internal/modules/auth"
	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
)

type stubGeo struct{}

func (stubGeo) Search(context.Context, uuid.UUID, string, float64, float64, int, uuid.UUID) ([]Place, error) {
	return []Place{{Label: "ok", Lat: 33.7, Lng: 72.3, Place: "Attock"}}, nil
}

func (stubGeo) Reverse(context.Context, uuid.UUID, float64, float64) (Place, error) {
	return Place{Label: "ok", Lat: 33.7, Lng: 72.3, Place: "Attock"}, nil
}

func TestSearchValidation(t *testing.T) {
	h := NewHandler(stubGeo{}, zerolog.Nop())
	accountID := uuid.New()
	cases := []struct {
		name string
		url  string
		code string
	}{
		{name: "short q", url: "/geo/search?q=ab", code: "invalid_input"},
		{name: "bad limit", url: "/geo/search?q=attock&limit=9", code: "invalid_input"},
		{name: "bad near", url: "/geo/search?q=attock&near_lat=999&near_lng=1", code: "invalid_input"},
		{name: "ok", url: "/geo/search?q=attock&limit=3", code: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{AccountID: accountID, Role: auth.RoleCustomer}))
			rec := httptest.NewRecorder()
			h.Search(rec, req)
			if tc.code == "" {
				if rec.Code != http.StatusOK {
					t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
				}
				return
			}
			assertErrorCode(t, rec, tc.code)
		})
	}
}

func TestReverseValidation(t *testing.T) {
	h := NewHandler(stubGeo{}, zerolog.Nop())
	accountID := uuid.New()
	cases := []struct {
		name string
		url  string
		code string
	}{
		{name: "missing lat", url: "/geo/reverse?lng=72.3", code: "invalid_input"},
		{name: "bad lng", url: "/geo/reverse?lat=33.7&lng=abc", code: "invalid_input"},
		{name: "out of range", url: "/geo/reverse?lat=91&lng=72.3", code: "invalid_input"},
		{name: "ok", url: "/geo/reverse?lat=33.7&lng=72.3", code: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{AccountID: accountID, Role: auth.RoleCustomer}))
			rec := httptest.NewRecorder()
			h.Reverse(rec, req)
			if tc.code == "" {
				if rec.Code != http.StatusOK {
					t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
				}
				return
			}
			assertErrorCode(t, rec, tc.code)
		})
	}
}

func assertErrorCode(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	if body.Error.Code != want {
		t.Fatalf("code = %q want %q status=%d", body.Error.Code, want, rec.Code)
	}
	if rec.Code == http.StatusOK {
		t.Fatal(apperror.Invalid("expected error response"))
	}
}
