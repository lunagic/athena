package athena

import (
	"context"

	"github.com/lunagic/athena/athenaservices/queue"
)

func WithQueue[T any](
	ctx context.Context,
	q queue.Queue[T],
	handler func(
		ctx context.Context,
		payload T,
	) error,
) ConfigurationFunc {
	return func(app *App) error {
		go func() {
			_ = q.Consume(ctx, handler)
		}()
		return nil
	}
}
