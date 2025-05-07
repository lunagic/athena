package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

func NewRepository[Key comparable, Value any](
	driver Driver,
	prefix string,
) *Repository[Key, Value] {
	return &Repository[Key, Value]{
		driver: driver,
		prefix: prefix,
	}
}

type Repository[Key comparable, Value any] struct {
	driver Driver
	prefix string
}

func (r *Repository[Key, Value]) Set(ctx context.Context, key Key, value Value, duration time.Duration) error {
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return err
	}

	if err := r.driver.Set(
		ctx,
		fmt.Sprintf("%s-%v", r.prefix, key),
		string(jsonBytes),
		duration,
	); err != nil {
		return err
	}

	return nil
}

func (r *Repository[Key, Value]) Get(ctx context.Context, key Key) (Value, error) {
	val, err := r.driver.Get(
		ctx,
		fmt.Sprintf("%s-%v", r.prefix, key),
	)
	if err != nil {
		return *new(Value), err
	}

	target := *new(Value)
	if err := json.Unmarshal([]byte(val), &target); err != nil {
		return *new(Value), err
	}

	return target, nil
}
