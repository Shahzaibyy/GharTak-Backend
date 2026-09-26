package admin_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/yourusername/ghartak-backend/internal/modules/admin"
	"github.com/yourusername/ghartak-backend/internal/platform/money"
)

type fakeZones struct {
	zones  []admin.Zone
	err    error
	active bool
	called bool
}

func (f *fakeZones) ListZones(_ context.Context, activeOnly bool) ([]admin.Zone, error) {
	f.called = true
	f.active = activeOnly
	return f.zones, f.err
}

func TestListZones(t *testing.T) {
	zoneID := uuid.MustParse("11111111-1111-4111-8111-111111111101")
	fee, err := money.Parse("60.00")
	if err != nil {
		t.Fatal(err)
	}
	rate, err := money.Parse("18.00")
	if err != nil {
		t.Fatal(err)
	}
	sample := []admin.Zone{{
		ID:              zoneID,
		CityName:        "Attock City",
		Slug:            "attock-city",
		BaseDeliveryFee: fee,
		PerKmRate:       rate,
		SurgeMultiplier: "1.00",
		ServiceRadiusKm: "8.00",
		IsActive:        true,
	}}
	tests := []struct {
		name       string
		query      string
		lister     *fakeZones
		status     int
		wantActive bool
		wantCalled bool
	}{
		{name: "all", lister: &fakeZones{zones: sample}, status: http.StatusOK, wantCalled: true},
		{name: "active", query: "?active=true", lister: &fakeZones{zones: sample}, status: http.StatusOK, wantActive: true, wantCalled: true},
		{name: "inactive flag", query: "?active=false", lister: &fakeZones{zones: sample}, status: http.StatusOK, wantCalled: true},
		{name: "bad flag", query: "?active=maybe", lister: &fakeZones{}, status: http.StatusBadRequest},
		{name: "repo error", lister: &fakeZones{err: errors.New("db")}, status: http.StatusInternalServerError, wantCalled: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := admin.NewHandler(tt.lister, zerolog.Nop())
			req := httptest.NewRequest(http.MethodGet, "/zones"+tt.query, nil)
			rec := httptest.NewRecorder()
			handler.ListZones(rec, req)
			if rec.Code != tt.status {
				t.Fatalf("status = %d body %s", rec.Code, rec.Body.String())
			}
			if tt.lister.called != tt.wantCalled || tt.lister.active != tt.wantActive {
				t.Fatalf("called %v active %v", tt.lister.called, tt.lister.active)
			}
		})
	}
}

func TestListZonesJSON(t *testing.T) {
	fee, _ := money.Parse("60.00")
	lister := &fakeZones{zones: []admin.Zone{{
		ID:              uuid.MustParse("11111111-1111-4111-8111-111111111101"),
		CityName:        "Attock City",
		Slug:            "attock-city",
		BaseDeliveryFee: fee,
		PerKmRate:       fee,
		SurgeMultiplier: "1.00",
		ServiceRadiusKm: "8.00",
		IsActive:        true,
	}}}
	handler := admin.NewHandler(lister, zerolog.Nop())
	req := httptest.NewRequest(http.MethodGet, "/zones", nil)
	rec := httptest.NewRecorder()
	handler.ListZones(rec, req)
	var body struct {
		Data []struct {
			CityName        string `json:"city_name"`
			BaseDeliveryFee string `json:"base_delivery_fee"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data) != 1 || body.Data[0].CityName != "Attock City" || body.Data[0].BaseDeliveryFee != "60.00" {
		t.Fatalf("body = %+v", body.Data)
	}
}
