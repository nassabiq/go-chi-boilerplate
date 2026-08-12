package cache

import (
	"context"

	"github.com/nassabiq/go-chi-boilerplate/internal/config"
	"github.com/redis/go-redis/v9"
)

func NewRedis(cfg *config.Config) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Address(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}

	return client, nil
}
