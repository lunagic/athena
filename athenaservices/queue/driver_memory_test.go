package queue_test

import (
	"testing"

	"github.com/lunagic/athena/athenaservices/queue"
	"gotest.tools/v3/assert"
)

func TestDriverMemory(t *testing.T) {
	driver, err := queue.NewDriverMemory()
	assert.NilError(t, err)
	testSuite(t, driver)
}
