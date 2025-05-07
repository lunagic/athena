package queue

import (
	"context"
	"errors"
)

func NewDriverMemory() (Driver, error) {
	return &driverMemory{
		queues: map[string]chan []byte{},
	}, nil
}

type driverMemory struct {
	queues map[string]chan []byte
}

func (driver *driverMemory) CreateQueue(ctx context.Context, queueName string) error {
	driver.queues[queueName] = make(chan []byte)
	return nil
}

func (driver *driverMemory) Publish(ctx context.Context, queueName string, payload []byte) error {
	// make sure queue exists
	queue, found := driver.queues[queueName]
	if !found {
		return errors.New("queue does not exist")
	}

	go func() {
		queue <- payload
	}()

	return nil
}

func (driver *driverMemory) Consume(
	ctx context.Context,
	queueName string,
	handler func(ctx context.Context, payload []byte) error,
) error {
	queue, found := driver.queues[queueName]
	if !found {
		return errors.New("queue does not exist")
	}

	for item := range queue {
		if err := handler(ctx, item); err != nil {
			return err
		}
	}

	return nil
}
