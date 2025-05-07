package cache_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lunagic/athena/athenaservices/cache"
	"gotest.tools/v3/assert"
)

func testCase(t *testing.T, driver cache.Driver) {
	key := uuid.NewString()
	value := uuid.NewString()

	{ // Confirm not found error
		_, err := driver.Get(t.Context(), key)
		assert.ErrorIs(t, err, cache.ErrNotFound)
	}

	{ // Confirm set
		if err := driver.Set(t.Context(), key, value, time.Second*30); err != nil {
			t.Fatal(err)
		}
	}

	{ // Confirm getting value works
		actualValue, err := driver.Get(t.Context(), key)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, actualValue, value)
	}

	{ // Delete
		if err := driver.Delete(t.Context(), key); err != nil {
			t.Fatal(err)
		}

		_, err := driver.Get(t.Context(), key)
		assert.ErrorIs(t, err, cache.ErrNotFound)
	}

	{ // Confirm Expiration
		key = uuid.NewString()
		value = uuid.NewString()

		// Set the value
		if err := driver.Set(t.Context(), key, value, time.Second*1); err != nil {
			t.Fatal(err)
		}

		// Confirm we can fetch the value
		actualValue, err := driver.Get(t.Context(), key)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, actualValue, value)

		// Wait for the context to expire
		time.Sleep(time.Second * 2)

		// Get the value and we expect it to fail
		_, expiredCheckErr := driver.Get(t.Context(), key)
		assert.ErrorIs(t, expiredCheckErr, cache.ErrNotFound)
	}
}
