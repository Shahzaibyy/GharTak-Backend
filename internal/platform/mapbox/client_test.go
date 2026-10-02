package mapbox_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yourusername/ghartak-backend/internal/platform/mapbox"
)

func TestRouteSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/directions/v5/mapbox/driving/") {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("geometries") != "geojson" {
			t.Fatal("geometries")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "Ok",
			"routes": []map[string]any{{
				"distance": 3420.4,
				"duration": 660.2,
				"geometry": map[string]any{"type": "LineString", "coordinates": [][]float64{{72.3, 33.7}, {72.4, 33.8}}},
			}},
		})
	}))
	defer server.Close()
	client := mapbox.NewClient(mapbox.Settings{Token: "test", BaseURL: server.URL, Timeout: time.Second})
	route, err := client.Route(context.Background(), mapbox.Point{Lat: 33.7, Lng: 72.3}, mapbox.Point{Lat: 33.8, Lng: 72.4})
	if err != nil {
		t.Fatal(err)
	}
	if route.DistanceM != 3420 || route.DurationS != 660 || len(route.Geometry) == 0 {
		t.Fatalf("route = %+v", route)
	}
}

func TestRouteRateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	client := mapbox.NewClient(mapbox.Settings{Token: "test", BaseURL: server.URL})
	_, err := client.Route(context.Background(), mapbox.Point{Lat: 33.7, Lng: 72.3}, mapbox.Point{Lat: 33.8, Lng: 72.4})
	if err != mapbox.ErrRateLimited {
		t.Fatalf("err = %v", err)
	}
}

func TestRouteNoRoute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "NoRoute", "routes": []any{}})
	}))
	defer server.Close()
	client := mapbox.NewClient(mapbox.Settings{Token: "test", BaseURL: server.URL})
	_, err := client.Route(context.Background(), mapbox.Point{Lat: 33.7, Lng: 72.3}, mapbox.Point{Lat: 33.8, Lng: 72.4})
	if err != mapbox.ErrNoRoute {
		t.Fatalf("err = %v", err)
	}
}

func TestFallbackOnUnavailable(t *testing.T) {
	inner := mapbox.NewFallbackRouter(failRouter{}, 1.3, 20)
	route, err := inner.Route(context.Background(), mapbox.Point{Lat: 33.7, Lng: 72.3}, mapbox.Point{Lat: 33.71, Lng: 72.31})
	if err != nil || !route.Approximate || route.DistanceM <= 0 {
		t.Fatalf("route = %+v err = %v", route, err)
	}
}

func TestSearchParses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("country") != "pk" {
			t.Fatal("country")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"features": []map[string]any{{
				"properties": map[string]any{
					"name":            "Attock",
					"full_address":    "Attock, Punjab, Pakistan",
					"place_formatted": "Attock, Punjab",
					"coordinates":     map[string]any{"longitude": 72.3667, "latitude": 33.7667},
				},
			}},
		})
	}))
	defer server.Close()
	client := mapbox.NewClient(mapbox.Settings{Token: "test", BaseURL: server.URL})
	list, err := client.Search(context.Background(), "attock", mapbox.Point{Lat: 33.7, Lng: 72.3}, 5)
	if err != nil || len(list) != 1 || list[0].Point.Lat != 33.7667 {
		t.Fatalf("list = %+v err = %v", list, err)
	}
}

func TestEmptyTokenUnavailable(t *testing.T) {
	client := mapbox.NewClient(mapbox.Settings{})
	_, err := client.Route(context.Background(), mapbox.Point{Lat: 1, Lng: 1}, mapbox.Point{Lat: 2, Lng: 2})
	if err != mapbox.ErrUnavailable {
		t.Fatalf("err = %v", err)
	}
}

type failRouter struct{}

func (failRouter) Route(context.Context, mapbox.Point, mapbox.Point) (mapbox.Route, error) {
	return mapbox.Route{}, mapbox.ErrUpstreamUnavailable
}
