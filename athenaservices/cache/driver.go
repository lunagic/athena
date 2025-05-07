package cache

// Recall?

import (
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("not found")

type Driver interface {
	Delete(ctx context.Context, key string) error
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, duration time.Duration) error
}
