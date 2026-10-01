package queue

import (
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

func RedisOpt(url string) (asynq.RedisClientOpt, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return asynq.RedisClientOpt{}, fmt.Errorf("queue: parse redis url: %w", err)
	}
	return asynq.RedisClientOpt{
		Addr:      opts.Addr,
		Password:  opts.Password,
		DB:        opts.DB,
		TLSConfig: opts.TLSConfig,
	}, nil
}
