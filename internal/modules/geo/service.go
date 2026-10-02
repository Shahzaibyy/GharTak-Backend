package geo

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/mapbox"
)

type ZoneProvider func(ctx context.Context, id uuid.UUID) (lat, lng float64, err error)

type limiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) error
}

type Service struct {
	geocoder mapbox.Geocoder
	zones    ZoneProvider
	limits   limiter
}

func NewService(geocoder mapbox.Geocoder, zones ZoneProvider, limits limiter) *Service {
	return &Service{geocoder: geocoder, zones: zones, limits: limits}
}

type Place struct {
	Label string  `json:"label"`
	Lat   float64 `json:"lat"`
	Lng   float64 `json:"lng"`
	Place string  `json:"place"`
}

func (s *Service) Search(ctx context.Context, accountID uuid.UUID, q string, nearLat, nearLng float64, limit int, zoneID uuid.UUID) ([]Place, error) {
	if err := s.allow(ctx, "geo:search:"+accountID.String(), 30); err != nil {
		return nil, err
	}
	near, err := s.near(ctx, nearLat, nearLng, zoneID)
	if err != nil {
		return nil, err
	}
	list, err := s.geocoder.Search(ctx, strings.TrimSpace(q), near, limit)
	if err != nil {
		return nil, mapGeoErr(err)
	}
	return toPlaces(list), nil
}

func (s *Service) Reverse(ctx context.Context, accountID uuid.UUID, lat, lng float64) (Place, error) {
	if err := s.allow(ctx, "geo:reverse:"+accountID.String(), 20); err != nil {
		return Place{}, err
	}
	result, err := s.geocoder.Reverse(ctx, mapbox.Point{Lat: lat, Lng: lng})
	if err != nil {
		return Place{}, mapGeoErr(err)
	}
	return Place{Label: result.Label, Lat: result.Point.Lat, Lng: result.Point.Lng, Place: result.Place}, nil
}

func (s *Service) near(ctx context.Context, lat, lng float64, zoneID uuid.UUID) (mapbox.Point, error) {
	if lat != 0 || lng != 0 {
		return mapbox.Point{Lat: lat, Lng: lng}, nil
	}
	if s.zones == nil || zoneID == uuid.Nil {
		return mapbox.Point{}, nil
	}
	zLat, zLng, err := s.zones(ctx, zoneID)
	if err != nil {
		return mapbox.Point{}, err
	}
	return mapbox.Point{Lat: zLat, Lng: zLng}, nil
}

func (s *Service) allow(ctx context.Context, key string, limit int) error {
	if s.limits == nil {
		return nil
	}
	return s.limits.Allow(ctx, key, limit, time.Minute)
}

func toPlaces(list []mapbox.GeoResult) []Place {
	out := make([]Place, 0, len(list))
	for _, item := range list {
		out = append(out, Place{Label: item.Label, Lat: item.Point.Lat, Lng: item.Point.Lng, Place: item.Place})
	}
	return out
}

func mapGeoErr(err error) error {
	if err == mapbox.ErrBadInput {
		return apperror.Invalid("geo query is invalid")
	}
	if err == mapbox.ErrNoRoute {
		return apperror.ErrNotFound
	}
	if err == mapbox.ErrUnavailable || err == mapbox.ErrUpstreamUnavailable || err == mapbox.ErrRateLimited {
		return apperror.ErrUnavailable
	}
	return err
}

// RedisLimiter wraps Redis INCR rate limits for geo endpoints.
type RedisLimiter struct {
	client *redis.Client
}

func NewRedisLimiter(client *redis.Client) *RedisLimiter {
	return &RedisLimiter{client: client}
}

func (l *RedisLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) error {
	count, err := l.client.Incr(ctx, key).Result()
	if err != nil {
		return err
	}
	if count == 1 {
		_ = l.client.Expire(ctx, key, window).Err()
	}
	if count > int64(limit) {
		return apperror.ErrRateLimited
	}
	return nil
}
