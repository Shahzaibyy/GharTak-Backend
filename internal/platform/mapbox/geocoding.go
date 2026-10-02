package mapbox

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/yourusername/ghartak-backend/internal/platform/geo"
)

type geocodeResponse struct {
	Features []geocodeFeature `json:"features"`
}

type geocodeFeature struct {
	Properties struct {
		Name           string `json:"name"`
		FullAddress    string `json:"full_address"`
		PlaceFormatted string `json:"place_formatted"`
		Coordinates    struct {
			Longitude float64 `json:"longitude"`
			Latitude  float64 `json:"latitude"`
		} `json:"coordinates"`
	} `json:"properties"`
	Geometry struct {
		Coordinates []float64 `json:"coordinates"`
	} `json:"geometry"`
}

func (c *Client) Search(ctx context.Context, q string, near Point, limit int) ([]GeoResult, error) {
	q = strings.TrimSpace(q)
	if len(q) < 3 || len(q) > 100 {
		return nil, ErrBadInput
	}
	if limit < 1 || limit > 5 {
		limit = 5
	}
	query := url.Values{}
	query.Set("q", q)
	query.Set("country", "pk")
	query.Set("limit", strconv.Itoa(limit))
	query.Set("permanent", "false")
	if geo.ValidCoord(near.Lat, near.Lng) {
		query.Set("proximity", encodeCoord(near))
	}
	body, err := c.get(ctx, "/search/geocode/v6/forward", query)
	if err != nil {
		return nil, err
	}
	return parseFeatures(body)
}

func (c *Client) Reverse(ctx context.Context, p Point) (GeoResult, error) {
	if !geo.ValidCoord(p.Lat, p.Lng) {
		return GeoResult{}, ErrBadInput
	}
	query := url.Values{}
	query.Set("longitude", formatFloat(p.Lng))
	query.Set("latitude", formatFloat(p.Lat))
	query.Set("country", "pk")
	query.Set("permanent", "false")
	query.Set("limit", "1")
	body, err := c.get(ctx, "/search/geocode/v6/reverse", query)
	if err != nil {
		return GeoResult{}, err
	}
	list, err := parseFeatures(body)
	if err != nil {
		return GeoResult{}, err
	}
	if len(list) == 0 {
		return GeoResult{}, ErrNoRoute
	}
	return list[0], nil
}

func parseFeatures(body []byte) ([]GeoResult, error) {
	var resp geocodeResponse
	if err := decodeJSON(body, &resp); err != nil {
		return nil, err
	}
	out := make([]GeoResult, 0, len(resp.Features))
	for _, feature := range resp.Features {
		out = append(out, featureResult(feature))
	}
	return out, nil
}

func featureResult(feature geocodeFeature) GeoResult {
	lat, lng := feature.Properties.Coordinates.Latitude, feature.Properties.Coordinates.Longitude
	if lat == 0 && lng == 0 && len(feature.Geometry.Coordinates) >= 2 {
		lng, lat = feature.Geometry.Coordinates[0], feature.Geometry.Coordinates[1]
	}
	label := feature.Properties.FullAddress
	if label == "" {
		label = feature.Properties.Name
	}
	return GeoResult{
		Label: label,
		Point: Point{Lat: lat, Lng: lng},
		Place: feature.Properties.PlaceFormatted,
	}
}
