package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
)

type refreshRecord struct {
	AccountID string `json:"account_id"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	FamilyID  string `json:"family_id"`
}

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client}
}

func (s *RedisStore) Save(ctx context.Context, key, hash string, ttl time.Duration) error {
	if err := s.client.Set(ctx, key, hash, ttl).Err(); err != nil {
		return fmt.Errorf("auth: save otp: %w", err)
	}
	return nil
}

func (s *RedisStore) Load(ctx context.Context, key string) (string, error) {
	value, err := s.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", apperror.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("auth: load otp: %w", err)
	}
	return value, nil
}

func (s *RedisStore) Delete(ctx context.Context, key string) error {
	if err := s.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("auth: delete otp: %w", err)
	}
	return nil
}

func (s *RedisStore) Allow(ctx context.Context, key string, limit int, window time.Duration) error {
	count, err := s.client.Incr(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("auth: rate limit: %w", err)
	}
	if count == 1 {
		return s.expireFirst(ctx, key, window, count, limit)
	}
	return s.overLimit(count, limit)
}

func (s *RedisStore) expireFirst(ctx context.Context, key string, window time.Duration, count int64, limit int) error {
	if err := s.client.Expire(ctx, key, window).Err(); err != nil {
		return fmt.Errorf("auth: rate limit expire: %w", err)
	}
	return s.overLimit(count, limit)
}

func (s *RedisStore) overLimit(count int64, limit int) error {
	if count > int64(limit) {
		return apperror.ErrRateLimited
	}
	return nil
}

func (s *RedisStore) SaveRefresh(ctx context.Context, hash string, record refreshRecord, ttl time.Duration) error {
	body, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("auth: encode refresh: %w", err)
	}
	pipe := s.client.TxPipeline()
	pipe.Set(ctx, refreshKey(hash), body, ttl)
	pipe.SAdd(ctx, familyKey(record.FamilyID), hash)
	pipe.Expire(ctx, familyKey(record.FamilyID), ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("auth: save refresh: %w", err)
	}
	return nil
}

func (s *RedisStore) Use(ctx context.Context, hash string) (refreshRecord, error) {
	raw, err := s.client.GetDel(ctx, refreshKey(hash)).Bytes()
	if errors.Is(err, redis.Nil) {
		return refreshRecord{}, apperror.ErrNotFound
	}
	if err != nil {
		return refreshRecord{}, fmt.Errorf("auth: use refresh: %w", err)
	}
	return decodeRefresh(raw)
}

func (s *RedisStore) MarkSpent(ctx context.Context, hash, familyID string, ttl time.Duration) error {
	if err := s.client.Set(ctx, spentKey(hash), familyID, ttl).Err(); err != nil {
		return fmt.Errorf("auth: mark refresh spent: %w", err)
	}
	return nil
}

func (s *RedisStore) SpentFamily(ctx context.Context, hash string) (string, error) {
	familyID, err := s.client.Get(ctx, spentKey(hash)).Result()
	if errors.Is(err, redis.Nil) {
		return "", apperror.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("auth: spent refresh: %w", err)
	}
	return familyID, nil
}

func (s *RedisStore) Revoke(ctx context.Context, hash string) error {
	raw, err := s.client.Get(ctx, refreshKey(hash)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("auth: revoke refresh: %w", err)
	}
	return s.dropRefresh(ctx, hash, raw)
}

func (s *RedisStore) dropRefresh(ctx context.Context, hash string, raw []byte) error {
	record, err := decodeRefresh(raw)
	if err != nil {
		return err
	}
	pipe := s.client.TxPipeline()
	pipe.Del(ctx, refreshKey(hash))
	pipe.SRem(ctx, familyKey(record.FamilyID), hash)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("auth: revoke refresh: %w", err)
	}
	return nil
}

func (s *RedisStore) RevokeFamily(ctx context.Context, familyID string) error {
	hashes, err := s.client.SMembers(ctx, familyKey(familyID)).Result()
	if err != nil {
		return fmt.Errorf("auth: list refresh family: %w", err)
	}
	return s.deleteHashes(ctx, familyID, hashes)
}

func (s *RedisStore) deleteHashes(ctx context.Context, familyID string, hashes []string) error {
	keys := make([]string, 0, len(hashes)+1)
	for _, hash := range hashes {
		keys = append(keys, refreshKey(hash))
	}
	keys = append(keys, familyKey(familyID))
	if err := s.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("auth: revoke refresh family: %w", err)
	}
	return nil
}

func decodeRefresh(raw []byte) (refreshRecord, error) {
	var record refreshRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return refreshRecord{}, fmt.Errorf("auth: decode refresh: %w", err)
	}
	return record, nil
}

func refreshKey(hash string) string { return "refresh:" + hash }

func spentKey(hash string) string { return "spent:" + hash }

func familyKey(id string) string { return "family:" + id }
