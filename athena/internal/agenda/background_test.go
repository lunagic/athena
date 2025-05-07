package agenda_test

import (
	"context"
	"testing"
	"time"

	"github.com/lunagic/athena/athena"
	"github.com/lunagic/athena/athena/internal/agenda"
	"github.com/lunagic/athena/athenaservices/cache"
	"gotest.tools/v3/assert"
)

func TestStuff(t *testing.T) {
	cacheDriver, err := cache.NewDriverMemory()
	assert.NilError(t, err)
	jobCount := 0
	jobs := []agenda.Job{
		athena.NewBackgroundJob(time.Second, athena.BackgroundJobActionFunc(func(ctx context.Context) error {
			t.Log("job ran")
			jobCount++
			return nil
		})),
	}

	// Start a large number of background jobs simulating many servers running
	serversToStart := 50
	for range serversToStart {
		go func() {
			_ = agenda.Background(
				t.Context(),
				cacheDriver,
				jobs,
			)
		}()
	}

	backOff := time.Second * 3 // This needs to match to sleep in the code to see if it wins the claim
	expectedJobCount := 3

	time.Sleep(time.Second*time.Duration(expectedJobCount) + backOff)
	diff := jobCount - expectedJobCount
	if diff < 0 {
		diff *= -1
	}

	assert.Assert(t, diff <= 1) // Might be off by one or so depending on when in the second the test starts
}
