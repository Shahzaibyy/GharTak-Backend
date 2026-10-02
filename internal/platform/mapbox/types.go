package mapbox

import (
	"context"
	"encoding/json"
	"errors"
)

var (
	ErrUpstreamUnavailable = errors.New("mapbox unavailable")
	ErrRateLimited         = errors.New("mapbox rate limited")
	ErrNoRoute             = errors.New("mapbox no route")
	ErrBadInput            = errors.New("mapbox bad input")
	ErrUnavailable         = errors.New("mapbox not configured")
)

// Point is lng/lat for Mapbox calls. Coordinates are not money.
type Point struct {
	Lat float64
	Lng float64
}

// Route is a driving path between two points.
type Route struct {
	DistanceM   int             // meters
	DurationS   int             // seconds
	Geometry    json.RawMessage // GeoJSON LineString
	Approximate bool
}

// GeoResult is one geocoding hit. Temporary results are not persisted as geocoder output.
type GeoResult struct {
	Label string
	Point Point
	Place string
}

// Router returns road distance, ETA, and geometry.
type Router interface {
	Route(ctx context.Context, from, to Point) (Route, error)
}

// Geocoder searches and reverse-geocodes within Pakistan.
type Geocoder interface {
	Search(ctx context.Context, q string, near Point, limit int) ([]GeoResult, error)
	Reverse(ctx context.Context, p Point) (GeoResult, error)
}
