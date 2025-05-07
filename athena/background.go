package athena

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/lunagic/athena/athena/internal/agenda"
	"github.com/lunagic/athena/athenaservices/cache"
)

type BackgroundJobActionFunc func(ctx context.Context) error

func (f BackgroundJobActionFunc) Run(ctx context.Context) error {
	return f(ctx)
}

type BackgroundJobAction interface {
	Run(ctx context.Context) error
}

func NewBackgroundJob(interval time.Duration, action BackgroundJobAction) *BackgroundJob {
	return &BackgroundJob{
		lastRan:  time.Now().Truncate(interval),
		interval: interval,
		action:   action,
	}
}

type BackgroundJob struct {
	lastRan  time.Time
	interval time.Duration
	action   BackgroundJobAction
}

func (s *BackgroundJob) Run(ctx context.Context) {
	now := time.Now().Truncate(time.Second)

	if !s.shouldRun(now) {
		return
	}

	s.lastRan = now

	_ = s.action.Run(ctx)
}

func (s *BackgroundJob) shouldRun(now time.Time) bool {
	sinceLastRun := now.Sub(s.lastRan)
	if sinceLastRun >= s.interval {
		return true
	}

	// Exactly the right run
	return now.Add(s.interval).Truncate(s.interval).Sub(now) == s.interval
}

type primarySchedulerPayload struct {
	UUID      string
	CheckedIn time.Time
}

func background(ctx context.Context, c cache.Driver, jobs []agenda.Job) error {
	runUUID := uuid.NewString()

	cacheGetter := cache.NewRepository[string, primarySchedulerPayload](c, "athena-primary-scheduler")
	checkIn := func() error {
		return cacheGetter.Set(
			ctx,
			"data",
			primarySchedulerPayload{
				UUID:      runUUID,
				CheckedIn: time.Now(),
			},
			time.Hour,
		)
	}

	backOffTimeToHandleCollisions := time.Second * 3
	maxTimeWithoutCheckIn := time.Second * 6

	return agenda.EverySecond(
		ctx,
		func(ctx context.Context) error {
			existingCheckInData, err := cacheGetter.Get(ctx, "data")
			if err != nil {
				if !errors.Is(err, cache.ErrNotFound) {
					return err
				}
			}

			// Check to see if this instance should take over as primary
			if existingCheckInData.UUID != runUUID {
				// Exit early if the primary instance has checked in recently
				if time.Since(existingCheckInData.CheckedIn) <= maxTimeWithoutCheckIn {
					return nil
				}

				// Attempt to claim role of primary instance
				if err := checkIn(); err != nil {
					return err
				}

				// Wait for other server's attempts to check in to complete
				// (wait to see who wins in the check in)
				time.Sleep(backOffTimeToHandleCollisions)

				// We also just return here to start the loop over again
				return nil
			}

			// Check in so no other server tries to check in and take over
			if err := checkIn(); err != nil {
				return err
			}

			for _, job := range jobs {
				go job.Run(ctx)
			}

			return nil
		},
	)
}
