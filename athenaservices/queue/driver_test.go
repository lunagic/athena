package queue_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/lunagic/athena/athenaservices/queue"
	"gotest.tools/v3/assert"
)

func testSuite(t *testing.T, driver queue.Driver) {
	queueThing, err := queue.NewQueue[string](t.Context(), driver, uuid.NewString())
	assert.NilError(t, err)

	publishErr := queueThing.Publish(t.Context(), uuid.NewString())
	assert.NilError(t, publishErr)

	expectedError := errors.New(uuid.NewString())

	consumeErr := queueThing.Consume(
		t.Context(),
		func(ctx context.Context, payload string) error {
			return expectedError
		},
	)
	assert.Equal(t, expectedError, consumeErr)
}
