package dispatch

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

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
	return g.client.Set(ctx, riderKey(riderID), zoneID.String(), 45*time.Second).Err()
}

func (g *Geo) Remove(ctx context.Context, zoneID, riderID uuid.UUID) error {
	if err := g.client.ZRem(ctx, zoneKey(zoneID), riderID.String()).Err(); err != nil {
		return err
	}
	return g.client.Del(ctx, riderKey(riderID)).Err()
}

func (g *Geo) Nearest(ctx context.Context, zoneID uuid.UUID, lng, lat float64, km int, skip map[uuid.UUID]struct{}) (uuid.UUID, error) {
	locs, err := g.client.GeoSearchLocation(ctx, zoneKey(zoneID), &redis.GeoSearchLocationQuery{
		GeoSearchQuery: redis.GeoSearchQuery{
			Longitude: lng, Latitude: lat, Radius: float64(km), RadiusUnit: "km", Sort: "ASC", Count: 10,
		},
	}).Result()
	if err != nil {
		return uuid.Nil, err
	}
	return g.firstLive(ctx, locs, skip)
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
