package cache

import (
	"context"
	"sync"
	"time"
)

type recallItem struct {
	Key       string
	Value     string
	ExpiresAt time.Time
}

func NewDriverMemory() (Driver, error) {
	driver := &driverMemory{
		mutex: &sync.Mutex{},
		data:  map[string]recallItem{},
	}

	go func() {
		for range time.NewTicker(time.Minute).C {
			driver.cleanup()
		}
	}()

	return driver, nil
}

type driverMemory struct {
	mutex *sync.Mutex
	data  map[string]recallItem
}

func (driver *driverMemory) Delete(ctx context.Context, key string) error {
	driver.mutex.Lock()
	defer driver.mutex.Unlock()

	delete(driver.data, key)

	return nil
}

func (driver *driverMemory) Get(ctx context.Context, key string) (string, error) {
	driver.mutex.Lock()
	defer driver.mutex.Unlock()

	item, found := driver.data[key]
	if !found {
		return "", ErrNotFound
	}

	if time.Since(item.ExpiresAt) >= 0 {
		return "", ErrNotFound
	}

	return item.Value, nil
}

func (driver *driverMemory) Set(ctx context.Context, key string, value string, duration time.Duration) error {
	driver.mutex.Lock()
	defer driver.mutex.Unlock()

	driver.data[key] = recallItem{
		Key:       key,
		Value:     value,
		ExpiresAt: time.Now().Add(duration),
	}

	return nil
}

func (driver *driverMemory) cleanup() {
	driver.mutex.Lock()
	defer driver.mutex.Unlock()

	keysToDelete := []string{}

	for key, value := range driver.data {
		if !time.Now().After(value.ExpiresAt) {
			continue
		}

		keysToDelete = append(keysToDelete, key)
	}

	for _, key := range keysToDelete {
		delete(driver.data, key)
	}
}
