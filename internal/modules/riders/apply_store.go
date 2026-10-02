package riders

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
)

const applyTTL = 30 * time.Minute

type applyStore interface {
	SaveApply(ctx context.Context, lookup, zoneID string) error
	TakeApply(ctx context.Context, lookup string) (string, error)
}

type RedisApply struct {
	client *redis.Client
}

func NewRedisApply(client *redis.Client) *RedisApply {
	return &RedisApply{client: client}
}

func (s *RedisApply) SaveApply(ctx context.Context, lookup, zoneID string) error {
	return s.client.Set(ctx, applyKey(lookup), zoneID, applyTTL).Err()
}

func (s *RedisApply) TakeApply(ctx context.Context, lookup string) (string, error) {
	key := applyKey(lookup)
	zoneID, err := s.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", apperror.Unauthorized("account is not registered")
	}
	if err != nil {
		return "", err
	}
	_ = s.client.Del(ctx, key).Err()
	return zoneID, nil
}

func applyKey(lookup string) string {
	return "rider-apply:" + lookup
}

func ParseZoneID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.UUID{}, apperror.Unauthorized("account is not registered")
	}
	return id, nil
}
