package cache_test

import (
	"testing"

	"github.com/lunagic/athena/athenaservices/cache"
)

func TestDriverMemory(t *testing.T) {
	t.Parallel()

	driver, err := cache.NewDriverMemory()
	if err != nil {
		t.Fatal(err)
	}

	testCase(t, driver)
}
