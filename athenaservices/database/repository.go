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

func NewRepository[ID ~int64, T Entity](service *Service, baseModifiers ...func(ctx context.Context, t *T) (QueryModifier, error)) Repository[ID, T] {
	baseQuery, err := generateBaseQuery(*new(T))
	if err != nil {
		panic(err)
	}

	r := Repository[ID, T]{
		selector:      NewSelector[T](service, baseQuery),
		T:             new(T),
		BaseModifiers: baseModifiers,
	}

	value := reflect.ValueOf(r.T).Elem()
	for i := range value.NumField() {
		fieldValue := value.Field(i)
		fieldDefinition := value.Type().Field(i)

		if !fieldDefinition.IsExported() {
			continue
		}

		tag := utils.ParseTag(fieldDefinition.Tag)
		columnName := tag.Column
		if columnName == "" {
			continue
		}

		service.mapping[fieldValue.UnsafeAddr()] = columnName
	}

	return r
}

type Repository[ID ~int64, T Entity] struct {
	selector      Selector[T]
	T             *T
	BaseModifiers []func(ctx context.Context, t *T) (QueryModifier, error)
}

func (repository *Repository[ID, T]) SelectMultiple(ctx context.Context, mods ...QueryModifier) ([]T, error) {
	for _, mod := range repository.BaseModifiers {
		queryModifier, err := mod(ctx, repository.T)
		if err != nil {
			return nil, err
		}

		// Prepend the base modifiers
		mods = append([]QueryModifier{queryModifier}, mods...)
	}

	return repository.selector.SelectMultiple(ctx, mods...)
}

func (repository *Repository[ID, T]) SelectSingle(ctx context.Context, mods ...QueryModifier) (T, error) {
	for _, mod := range repository.BaseModifiers {
		queryModifier, err := mod(ctx, repository.T)
		if err != nil {
			return *new(T), err
		}

		// Prepend the base modifiers
		mods = append([]QueryModifier{queryModifier}, mods...)
	}
	return repository.selector.SelectSingle(ctx, mods...)
}

func (repository *Repository[ID, T]) Insert(ctx context.Context, entity T) (ID, error) {
	statement, err := repository.selector.service.driver.generateInsert(entity)
	if err != nil {
		return 0, err
	}

	if !repository.selector.service.driver.usesLastInsertId() {
		lastInsertID := []struct {
			ID ID `db:"id"`
		}{}
		if err := repository.selector.service.runSelect(ctx, statement, &lastInsertID); err != nil {
			return 0, err
		}

		return lastInsertID[0].ID, nil
	}

	result, err := repository.selector.service.runExecute(ctx, statement)
	if err != nil {
		return 0, err
	}

	lastInsertID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	if lastInsertID == 0 {
		if err := utils.LoopOverStructFields(reflect.ValueOf(entity), func(fieldDefinition reflect.StructField, fieldValue reflect.Value) error {
			// Grab the primary key if it wasn't from an AUTO_INCREMENT
			tag := utils.ParseTag(fieldDefinition.Tag)
			if tag.PrimaryKey {
				lastInsertID = fieldValue.Int()
			}

			return nil
		}); err != nil {
			return 0, nil
		}
	}

	return ID(lastInsertID), nil
}

func (repository *Repository[ID, T]) Update(ctx context.Context, entity T) error {
	statement, err := repository.selector.service.driver.generateUpdate(entity)
	if err != nil {
		return err
	}

	if _, err := repository.selector.service.runExecute(ctx, statement); err != nil {
		return err
	}

	return nil
}

func (repository *Repository[ID, T]) Delete(ctx context.Context, entity T) error {
	statement, err := repository.selector.service.driver.generateDelete(entity)
	if err != nil {
		return err
	}

	if _, err := repository.selector.service.runExecute(ctx, statement); err != nil {
		return err
	}

	return nil
}
