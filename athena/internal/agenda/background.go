package agenda

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/lunagic/athena/athenaservices/cache"
)

type PrimarySchedulerPayload struct {
	UUID      string
	CheckedIn time.Time
}

func Background(ctx context.Context, c cache.Driver, jobs []Job) error {
	runUUID := uuid.NewString()

	cacheGetter := cache.NewRepository[string, PrimarySchedulerPayload](c, "athena-primary-scheduler")
	checkIn := func() error {
		return cacheGetter.Set(
			ctx,
			"data",
			PrimarySchedulerPayload{
				UUID:      runUUID,
				CheckedIn: time.Now(),
			},
			time.Hour,
		)
	}

	backOffTimeToHandleCollisions := time.Second * 3
	maxTimeWithoutCheckIn := time.Second * 6

	return EverySecond(
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
