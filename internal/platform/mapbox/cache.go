package mapbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"

	"github.com/yourusername/ghartak-backend/internal/platform/geo"
)

type cacheStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
}

type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(client *redis.Client) *RedisCache {
	return &RedisCache{client: client}
}

func (c *RedisCache) Get(ctx context.Context, key string) (string, error) {
	value, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", ErrNoRoute
	}
	return value, err
}

func (c *RedisCache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

type CachedRouter struct {
	inner Router
	cache cacheStore
	ttl   time.Duration
	group singleflight.Group
}

func NewCachedRouter(inner Router, cache cacheStore, ttl time.Duration) *CachedRouter {
	if ttl <= 0 {
		ttl = 6 * time.Hour
	}
	return &CachedRouter{inner: inner, cache: cache, ttl: ttl}
}

func (r *CachedRouter) Route(ctx context.Context, from, to Point) (Route, error) {
	key := routeKey(from, to)
	if cached, err := r.load(ctx, key); err == nil {
		return cached, nil
	}
	return r.fetch(ctx, key, from, to)
}

func (r *CachedRouter) load(ctx context.Context, key string) (Route, error) {
	if r.cache == nil {
		return Route{}, ErrNoRoute
	}
	raw, err := r.cache.Get(ctx, key)
	if err != nil {
		return Route{}, err
	}
	var route Route
	if err := json.Unmarshal([]byte(raw), &route); err != nil {
		return Route{}, err
	}
	return route, nil
}

func (r *CachedRouter) fetch(ctx context.Context, key string, from, to Point) (Route, error) {
	value, err, _ := r.group.Do(key, func() (any, error) {
		route, err := r.inner.Route(ctx, from, to)
		if err != nil {
			return Route{}, err
		}
		r.save(ctx, key, route)
		return route, nil
	})
	if err != nil {
		return Route{}, err
	}
	return value.(Route), nil
}

func (r *CachedRouter) save(ctx context.Context, key string, route Route) {
	if r.cache == nil {
		return
	}
	raw, err := json.Marshal(route)
	if err != nil {
		return
	}
	_ = r.cache.Set(ctx, key, string(raw), r.ttl)
}

func routeKey(from, to Point) string {
	return fmt.Sprintf("route:%g:%g:%g:%g",
		geo.Round4(from.Lat), geo.Round4(from.Lng),
		geo.Round4(to.Lat), geo.Round4(to.Lng),
	)
}

// FallbackRouter uses Mapbox when available and haversine otherwise.
type FallbackRouter struct {
	inner       Router
	circuity    float64
	fallbackKMH float64
}

func NewFallbackRouter(inner Router, circuity, kmh float64) *FallbackRouter {
	if circuity < 1 {
		circuity = 1.3
	}
	if kmh <= 0 {
		kmh = 20
	}
	return &FallbackRouter{inner: inner, circuity: circuity, fallbackKMH: kmh}
}

func (r *FallbackRouter) Route(ctx context.Context, from, to Point) (Route, error) {
	if r.inner == nil {
		return r.estimate(from, to), nil
	}
	route, err := r.inner.Route(ctx, from, to)
	if err == nil {
		return route, nil
	}
	if shouldFallback(err) {
		return r.estimate(from, to), nil
	}
	return Route{}, err
}

func (r *FallbackRouter) estimate(from, to Point) Route {
	meters := geo.RoadEstimate(geo.Point{Lat: from.Lat, Lng: from.Lng}, geo.Point{Lat: to.Lat, Lng: to.Lng}, r.circuity)
	return Route{
		DistanceM:   int(meters),
		DurationS:   geo.DurationSeconds(meters, r.fallbackKMH),
		Approximate: true,
	}
}

func shouldFallback(err error) bool {
	return err == ErrRateLimited || err == ErrUpstreamUnavailable || err == ErrUnavailable
}
