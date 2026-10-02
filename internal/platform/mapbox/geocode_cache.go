package mapbox

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yourusername/ghartak-backend/internal/platform/geo"
)

type CachedGeocoder struct {
	inner      Geocoder
	cache      cacheStore
	searchTTL  time.Duration
	reverseTTL time.Duration
}

func NewCachedGeocoder(inner Geocoder, cache cacheStore, searchTTL, reverseTTL time.Duration) *CachedGeocoder {
	if searchTTL <= 0 {
		searchTTL = 24 * time.Hour
	}
	if reverseTTL <= 0 {
		reverseTTL = 7 * 24 * time.Hour
	}
	return &CachedGeocoder{inner: inner, cache: cache, searchTTL: searchTTL, reverseTTL: reverseTTL}
}

func (g *CachedGeocoder) Search(ctx context.Context, q string, near Point, limit int) ([]GeoResult, error) {
	key := searchKey(q, near)
	if list, err := g.loadList(ctx, key); err == nil {
		return list, nil
	}
	list, err := g.inner.Search(ctx, q, near, limit)
	if err != nil {
		return nil, err
	}
	g.save(ctx, key, list, g.searchTTL)
	return list, nil
}

func (g *CachedGeocoder) Reverse(ctx context.Context, p Point) (GeoResult, error) {
	key := reverseKey(p)
	if list, err := g.loadList(ctx, key); err == nil && len(list) > 0 {
		return list[0], nil
	}
	result, err := g.inner.Reverse(ctx, p)
	if err != nil {
		return GeoResult{}, err
	}
	g.save(ctx, key, []GeoResult{result}, g.reverseTTL)
	return result, nil
}

func (g *CachedGeocoder) loadList(ctx context.Context, key string) ([]GeoResult, error) {
	if g.cache == nil {
		return nil, ErrNoRoute
	}
	raw, err := g.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	var list []GeoResult
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return nil, err
	}
	return list, nil
}

func (g *CachedGeocoder) save(ctx context.Context, key string, list []GeoResult, ttl time.Duration) {
	if g.cache == nil {
		return
	}
	raw, err := json.Marshal(list)
	if err != nil {
		return
	}
	_ = g.cache.Set(ctx, key, string(raw), ttl)
}

func searchKey(q string, near Point) string {
	sum := sha1.Sum([]byte(strings.ToLower(strings.TrimSpace(q))))
	return fmt.Sprintf("geo:s:%s:%g:%g", hex.EncodeToString(sum[:]), geo.Round2(near.Lat), geo.Round2(near.Lng))
}

func reverseKey(p Point) string {
	return fmt.Sprintf("geo:r:%g:%g", geo.Round4(p.Lat), geo.Round4(p.Lng))
}
