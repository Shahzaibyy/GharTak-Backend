package dispatch

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const presenceTTL = 2 * time.Minute

type Geo struct {
	client *redis.Client
}

func NewGeo(client *redis.Client) *Geo {
	return &Geo{client: client}
}

func (g *Geo) Online(ctx context.Context, zoneID, riderID uuid.UUID, lat, lng float64) error {
	return g.Upsert(ctx, zoneID, riderID, lng, lat)
}

func (g *Geo) Offline(ctx context.Context, zoneID, riderID uuid.UUID) error {
	return g.Remove(ctx, zoneID, riderID)
}

func (g *Geo) Upsert(ctx context.Context, zoneID, riderID uuid.UUID, lng, lat float64) error {
	key := zoneKey(zoneID)
	_, err := g.client.GeoAdd(ctx, key, &redis.GeoLocation{Name: riderID.String(), Longitude: lng, Latitude: lat}).Result()
	if err != nil {
		return err
	}
	if err := g.client.Expire(ctx, key, 24*time.Hour).Err(); err != nil {
		return err
	}
	return g.client.Set(ctx, riderKey(riderID), zoneID.String(), presenceTTL).Err()
}

func (g *Geo) Remove(ctx context.Context, zoneID, riderID uuid.UUID) error {
	if err := g.client.ZRem(ctx, zoneKey(zoneID), riderID.String()).Err(); err != nil {
		return err
	}
	return g.client.Del(ctx, riderKey(riderID)).Err()
}

func (g *Geo) Nearest(ctx context.Context, zoneID uuid.UUID, lng, lat float64, km int, skip map[uuid.UUID]struct{}) (uuid.UUID, error) {
	locs, err := g.search(ctx, zoneID, lng, lat, km)
	if err != nil {
		return uuid.Nil, err
	}
	return g.firstLive(ctx, locs, skip)
}

// search prefers GEOSEARCH (Redis 6.2+) and falls back to GEORADIUS for older Redis
// (common on free-tier hosts where GEOADD works but GEOSEARCH returns unknown command).
func (g *Geo) search(ctx context.Context, zoneID uuid.UUID, lng, lat float64, km int) ([]redis.GeoLocation, error) {
	key := zoneKey(zoneID)
	locs, err := g.client.GeoSearchLocation(ctx, key, &redis.GeoSearchLocationQuery{
		GeoSearchQuery: redis.GeoSearchQuery{
			Longitude: lng, Latitude: lat, Radius: float64(km), RadiusUnit: "km", Sort: "ASC", Count: 10,
		},
	}).Result()
	if err == nil || errors.Is(err, redis.Nil) {
		return locs, nil
	}
	return g.client.GeoRadius(ctx, key, lng, lat, &redis.GeoRadiusQuery{
		Radius: float64(km), Unit: "km", Sort: "ASC", Count: 10,
	}).Result()
}

func (g *Geo) firstLive(ctx context.Context, locs []redis.GeoLocation, skip map[uuid.UUID]struct{}) (uuid.UUID, error) {
	for _, loc := range locs {
		id, ok, err := g.usable(ctx, loc.Name, skip)
		if err != nil {
			return uuid.Nil, err
		}
		if ok {
			return id, nil
		}
	}
	return uuid.Nil, nil
}

func (g *Geo) usable(ctx context.Context, name string, skip map[uuid.UUID]struct{}) (uuid.UUID, bool, error) {
	id, err := uuid.Parse(name)
	if err != nil || skipped(skip, id) {
		return uuid.Nil, false, nil
	}
	return g.live(ctx, id)
}

func (g *Geo) live(ctx context.Context, id uuid.UUID) (uuid.UUID, bool, error) {
	n, err := g.client.Exists(ctx, riderKey(id)).Result()
	if err != nil {
		return uuid.Nil, false, err
	}
	return id, n == 1, nil
}

func skipped(skip map[uuid.UUID]struct{}, id uuid.UUID) bool {
	_, ok := skip[id]
	return ok
}

func zoneKey(id uuid.UUID) string { return "geo:zone:" + id.String() }

func riderKey(id uuid.UUID) string { return "geo:rider:" + id.String() }
