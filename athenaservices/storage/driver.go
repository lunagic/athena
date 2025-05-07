package storage

import (
	"context"
	"io"
	"time"
)

type Driver interface {
	Put(ctx context.Context, filePath string, payload io.Reader) error
	Get(ctx context.Context, filePath string) (io.Reader, error)
	Delete(ctx context.Context, filePath string) error
	Exists(ctx context.Context, filePath string) (bool, error)
	IsReady(ctx context.Context) error
	PreSignedURL(ctx context.Context, filePath string, expiration time.Duration) (string, error)
	PublicLink(ctx context.Context, filePath string) (string, error)
}
