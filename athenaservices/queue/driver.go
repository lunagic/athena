package queue

import (
	"context"
	"encoding/json"
)

type Driver interface {
	CreateQueue(ctx context.Context, queueName string) error
	Publish(ctx context.Context, queueName string, payload []byte) error
	Consume(ctx context.Context, queueName string, handler func(ctx context.Context, payload []byte) error) error
}

func NewQueue[T any](ctx context.Context, driver Driver, name string) (Queue[T], error) {
	if err := driver.CreateQueue(ctx, name); err != nil {
		return Queue[T]{}, err
	}

	return Queue[T]{
		driver: driver,
		name:   name,
	}, nil
}

type Queue[T any] struct {
	driver Driver
	name   string
}

func (q Queue[T]) Publish(ctx context.Context, message T) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}

	return q.driver.Publish(ctx, q.name, payload)
}

func (q Queue[T]) Consume(ctx context.Context, handler Handler[T]) error {
	return q.driver.Consume(ctx, q.name, func(ctx context.Context, payload []byte) error {
		target := *new(T)
		if err := json.Unmarshal(payload, &target); err != nil {
			return err
		}

		return handler(ctx, target)
	})
}

type Handler[T any] func(ctx context.Context, payload T) error
