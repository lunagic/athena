package athena

import (
	"context"
	"errors"
	"time"

	"github.com/lunagic/athena/athena/internal/agenda"
	"github.com/lunagic/athena/athenaservices/cache"
)

type BackgroundJob struct {
	name     string
	interval time.Duration
	action   func(ctx context.Context)
}

type primarySchedulerPayload struct {
	UUID      string
	CheckedIn time.Time
}

func NewBackgroundJob(name string, interval time.Duration, action func(ctx context.Context)) BackgroundJob {
	return BackgroundJob{
		name:     name,
		interval: interval,
		action:   action,
	}
}

func WithBackgroundJobs(cacheDriver cache.Driver, jobs []BackgroundJob) ConfigurationFunc {
	return func(app *App) error {
		app.jobsCacheService = cacheDriver
		app.jobs = jobs

		return nil
	}
}

func (app *App) Background(ctx context.Context) error {
	cacheGetter := cache.NewRepository[string, primarySchedulerPayload](app.jobsCacheService, "athena-primary-scheduler")
	jobLastRunTracker := cache.NewRepository[string, time.Time](app.jobsCacheService, "athena-job-last-ran")

	checkIn := func() error {
		return cacheGetter.Set(
			ctx,
			"data",
			primarySchedulerPayload{
				UUID:      app.instanceUUID,
				CheckedIn: time.Now(),
			},
			time.Hour,
		)
	}

	backOffTimeToHandleCollisions := time.Second * 3
	maxTimeWithoutCheckIn := time.Second * 6

	var primaryServerChecker func(checkNumber int) bool
	primaryServerChecker = func(checkNumber int) bool {
		existingCheckInData, err := cacheGetter.Get(ctx, "data")
		if err != nil {
			if !errors.Is(err, cache.ErrNotFound) {
				return false
			}
		}

		// Check to see if this instance should take over as primary
		if existingCheckInData.UUID != app.instanceUUID {
			// Exit early if the primary instance has checked in recently
			if time.Since(existingCheckInData.CheckedIn) <= maxTimeWithoutCheckIn {
				return false
			}

			// Attempt to claim role of primary instance
			if err := checkIn(); err != nil {
				return false
			}

			// Wait for other server's attempts to check in to complete
			// (wait to see who wins in the check in)
			time.Sleep(backOffTimeToHandleCollisions)

			// We also just return here to start the loop over again
			// this could return in us being the primary the next time this function runs
			return primaryServerChecker(checkNumber + 1)
		}

		return true
	}

	jobCanRun := func(ctx context.Context, job BackgroundJob) bool {
		lastRan, err := jobLastRunTracker.Get(ctx, job.name)
		if err != nil {
			return errors.Is(err, cache.ErrNotFound)
		}

		// Skip if it's been too soon
		if time.Since(lastRan) <= job.interval {
			return false
		}

		// if time.Now().Truncate(job.interval).Sub(time.Now().Truncate(time.Second)) == 0 {

		// }

		return true
	}

	for _, job := range app.jobs {
		go func() {
			_ = agenda.Interval(
				ctx,
				job.interval,
				func(ctx context.Context) error {
					// Check if we should be the primary server
					// If we are not the primary instance so we do nothing
					if !primaryServerChecker(1) {
						return nil
					}

					// Check in so no other server tries to check in and take over
					if err := checkIn(); err != nil {
						return err
					}

					// Kick off all the jobs
					if !jobCanRun(ctx, job) {
						return nil
					}

					// Track the job as ran
					if err := jobLastRunTracker.Set(ctx, job.name, time.Now(), job.interval*2); err != nil {
						return err
					}

					// Run the job
					go job.action(ctx)

					return nil
				},
				func(ctx context.Context, err error) error {
					return nil
				},
			)
		}()
	}

	return nil
}
