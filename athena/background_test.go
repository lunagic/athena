package athena_test

import (
	"context"
	"testing"
	"time"

	"github.com/lunagic/athena/athena"
	"gotest.tools/v3/assert"
)

func TestBackgroundJobs(t *testing.T) {
	ctx := t.Context()
	config := athena.NewDefaultConfig()
	config.AppHTTPPort = 0

	cacheDriver, err := config.Cache()
	assert.NilError(t, err)

	// Set a counter that the background job will increment every time it runs
	counter := 0

	// Start multiple "Apps" with their own background job logic
	// This simulates the application running in multiple locations (servers)
	// Notice that we do share the same cache driver (this is important)
	for range 5 {
		go func() {
			app, err := athena.NewApp(
				ctx,
				config,
				athena.WithBackgroundJobs(
					cacheDriver,
					[]athena.BackgroundJob{
						athena.NewBackgroundJob("foobar", time.Minute, func(ctx context.Context) {
							counter++
						}),
					},
				),
			)
			assert.NilError(t, err)

			if err := app.Start(ctx); err != nil {
				t.Log(err)
			}
		}()
	}

	// Wait for a "primary" instance to be determined (takes about 3 seconds)
	time.Sleep(time.Second * 5)

	// Only one (the primary) instance should actually run the job
	// and it can only run it a max of 2 times
	// Once on determining that it's the primary (always happens)
	// Once if the minute changes over during the test (might happen)
	assert.Assert(t, counter == 1 || counter == 2)
}
