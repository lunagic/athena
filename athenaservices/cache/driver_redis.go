package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	redis "github.com/redis/go-redis/v9"
)

type DriverRedisConfig struct {
	Host   string
	Number int
	Pass   string
	Port   int
	User   string
}

func NewDriverRedis(config DriverRedisConfig) (Driver, error) {
	return &driverRedis{
		client: redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%d", config.Host, config.Port),
			Username: config.User,
			Password: config.Pass,
			DB:       config.Number,
		}),
	}, nil
}

type driverRedis struct {
	client *redis.Client
}

func (driver *driverRedis) Get(ctx context.Context, key string) (string, error) {
	result, err := driver.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", ErrNotFound
		}

		return "", err
	}

	return result, nil
}

func (driver *driverRedis) Set(ctx context.Context, key string, value string, duration time.Duration) error {
	return driver.client.Set(ctx, key, value, duration).Err()
}

func (driver *driverRedis) Delete(ctx context.Context, key string) error {
	return driver.client.Del(ctx, key).Err()
}
