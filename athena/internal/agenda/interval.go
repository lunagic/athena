package agenda

import (
	"context"
	"time"
)

func EveryHour(
	ctx context.Context,
	action func(ctx context.Context) error,
	errorHandler func(ctx context.Context, err error) error,
) error {
	return Interval(ctx, time.Hour, action, errorHandler)
}

func EveryMinute(
	ctx context.Context,
	action func(ctx context.Context) error,
	errorHandler func(ctx context.Context, err error) error,
) error {
	return Interval(ctx, time.Minute, action, errorHandler)
}

func EverySecond(
	ctx context.Context,
	action func(ctx context.Context) error,
	errorHandler func(ctx context.Context, err error) error,
) error {
	return Interval(ctx, time.Second, action, errorHandler)
}

func Interval(
	ctx context.Context,
	d time.Duration,
	action func(ctx context.Context) error,
	errorHandler func(ctx context.Context, err error) error,
) error {
	for {
		// Run right away
		if err := action(ctx); err != nil {
			if err := errorHandler(ctx, err); err != nil {
				return err
			}
		}

		// Wait for the correct amount of
		// time before running again
		now := time.Now()
		time.Sleep(now.Add(d).Truncate(d).Sub(now))
	}
}
