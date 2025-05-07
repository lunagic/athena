package agenda

import (
	"context"
	"time"
)

// func Test(ctx context.Context) {

// 	x := NewJob(time.Second, func(ctx context.Context) error {
// 		now := time.Now()
// 		log.Println(now.Truncate(time.Second).Sub(now))
// 		return nil
// 	})

// 	y := NewJob(time.Minute, func(ctx context.Context) error {
// 		now := time.Now()
// 		log.Println(now.Truncate(time.Second).Sub(now))
// 		return nil
// 	})

// 	EverySecond(ctx, func(ctx context.Context) error {
// 		x.Run(ctx)
// 		y.Run(ctx)
// 		return nil
// 	})

// 	time.Sleep(time.Hour)
// }

func EveryHour(ctx context.Context, action func(ctx context.Context) error) error {
	return Interval(ctx, time.Hour, action)
}

func EveryMinute(ctx context.Context, action func(ctx context.Context) error) error {
	return Interval(ctx, time.Minute, action)
}

func EverySecond(ctx context.Context, action func(ctx context.Context) error) error {
	return Interval(ctx, time.Second, action)
}

func Interval(ctx context.Context, d time.Duration, action func(ctx context.Context) error) error {
	for {
		now := time.Now()
		time.Sleep(now.Add(d).Truncate(d).Sub(now))
		if err := action(ctx); err != nil {
			return err
		}
	}
}

type Action func(ctx context.Context) error

type Job interface {
	Run(ctx context.Context)
}
