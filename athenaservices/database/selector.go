package database

// ================================================================
// ================================================================
// ============================= DONE =============================
// ================================================================
// ================================================================

import (
	"context"
	"reflect"

	"github.com/lunagic/athena/athenaservices/database/internal/utils"
)

func NewSelector[T any](service *Service, baseQuery Query) Selector[T] {
	return Selector[T]{
		service:   service,
		baseQuery: baseQuery,
	}
}

type Selector[T any] struct {
	service   *Service
	baseQuery Query
}

type QueryModifier func(query Query) Query

func WithLimitOverride(size int, offset int) QueryModifier {
	return func(query Query) Query {
		query.Limit.Count = size
		query.Limit.Offset = offset

		return query
	}
}

func WithAdditionalWhere(where OperatorOfLogic) QueryModifier {
	return func(query Query) Query {
		if query.Where == nil {
			query.Where = where
		} else {
			query.Where = And(query.Where, where)
		}

		return query
	}
}

func (selector *Selector[T]) SelectMultiple(ctx context.Context, mods ...QueryModifier) ([]T, error) {
	target := []T{}

	query := selector.baseQuery
	for _, mod := range mods {
		query = mod(query)
	}

	statement, err := selector.service.driver.generateSelect(query)
	if err != nil {
		return nil, err
	}

	if err := selector.service.runSelect(ctx, statement, &target); err != nil {
		return nil, err
	}

	return target, nil
}

func (selector *Selector[T]) SelectSingle(ctx context.Context, mods ...QueryModifier) (T, error) {
	mods = append(mods, WithLimitOverride(1, 0))

	rows, err := selector.SelectMultiple(ctx, mods...)
	if err != nil {
		return *new(T), err
	}

	if len(rows) < 1 {
		return *new(T), ErrNoRows
	}

	return rows[0], nil
}

func generateBaseQuery(e Entity) (Query, error) {
	selects := []string{}
	if err := utils.LoopOverStructFields(reflect.ValueOf(e), func(fieldDefinition reflect.StructField, fieldValue reflect.Value) error {
		tag := utils.ParseTag(fieldDefinition.Tag)
		if tag.Column == "" {
			return nil
		}

		selects = append(selects, tag.Column)

		return nil
	}); err != nil {
		return Query{}, err
	}

	return Query{
		Select: selects,
		From:   e.TableStructure().Name,
	}, nil
}
