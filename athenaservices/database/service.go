package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/lunagic/athena/athenaservices/database/internal/utils"
)

type Service struct {
	driver            Driver
	standardLibraryDB *sql.DB
	preRunFuncs       []func(ctx context.Context, statement string, args []any) error
	postRunFuncs      []func(ctx context.Context) error
	mapping           map[uintptr]string
}

func New(
	driver Driver,
	configFuncs ...ServiceConfigFunc,
) (*Service, error) {
	db, err := driver.Open()
	if err != nil {
		return nil, err
	}

	service := &Service{
		driver:            driver,
		standardLibraryDB: db,
		preRunFuncs:       []func(ctx context.Context, statement string, args []any) error{},
		postRunFuncs:      []func(ctx context.Context) error{},
		mapping:           map[uintptr]string{},
	}

	driver.setMapping(service.mapping)

	for _, configFunc := range configFuncs {
		if err := configFunc(service); err != nil {
			return nil, err
		}
	}

	return service, nil
}

func (service *Service) Ping() error {
	return service.standardLibraryDB.Ping()
}

func (service *Service) runSelect(
	ctx context.Context,
	statement statement,
	targetPointer any,
) error {
	preparedQuery, preparedArgs, err := utils.Prepare(statement.Query, statement.Parameters, service.driver.usesNumberedParameters())
	if err != nil {
		return err
	}

	if preparedQuery == "" {
		return ErrBlankQuery
	}

	for _, preRunFunc := range service.preRunFuncs {
		if err := preRunFunc(ctx, preparedQuery, preparedArgs); err != nil {
			return err
		}
	}

	rows, err := service.standardLibraryDB.Query(preparedQuery, preparedArgs...)
	if err != nil {
		return err
	}
	defer func() {
		_ = rows.Close()
	}()

	columns, err := rows.Columns()
	if err != nil {
		return err
	}

	target := reflect.ValueOf(targetPointer).Elem()
	targetType := reflect.TypeOf(target.Interface()).Elem()

	fieldIndexesToUse := []int{}

	rowMap := map[string]int{}
	testRow := reflect.New(targetType).Elem()
	for i := range testRow.NumField() {
		fieldDefinition := testRow.Type().Field(i)
		if !fieldDefinition.IsExported() {
			continue
		}

		tag := utils.ParseTag(fieldDefinition.Tag)
		if tag.Column == "" {
			continue
		}

		rowMap[tag.Column] = i
	}

	for _, column := range columns {
		fieldIndex, found := rowMap[column]
		if !found {
			return fmt.Errorf("column %s not found in target", column)
		}

		fieldIndexesToUse = append(fieldIndexesToUse, fieldIndex)
	}

	for rows.Next() {
		row := reflect.New(targetType).Elem()

		scanFields := []any{}
		jsonMapping := map[int]*string{}
		for _, fieldIndexToUse := range fieldIndexesToUse {
			if shouldBeJson(testRow.Type().Field(fieldIndexToUse)) {
				// Swap in a pointer to a string when the field should be json so we can unmarshal it later
				jsonString := ""
				jsonMapping[fieldIndexToUse] = &jsonString
				scanFields = append(scanFields, &jsonString)
			} else {
				scanFields = append(scanFields, row.Field(fieldIndexToUse).Addr().Interface())
			}
		}

		if err := rows.Scan(scanFields...); err != nil {
			return err
		}

		for fieldIndexToUse, jsonString := range jsonMapping {
			if err := json.Unmarshal([]byte(*jsonString), row.Field(fieldIndexToUse).Addr().Interface()); err != nil {
				return err
			}
		}

		target.Set(reflect.Append(target, row))
	}

	for _, postRunFunc := range service.postRunFuncs {
		if err := postRunFunc(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (service *Service) runExecute(
	ctx context.Context,
	statement statement,
) (
	sql.Result,
	error,
) {
	preparedQuery, preparedArgs, err := utils.Prepare(statement.Query, statement.Parameters, service.driver.usesNumberedParameters())
	if err != nil {
		return nil, err
	}

	if preparedQuery == "" {
		return nil, ErrBlankQuery
	}

	for _, preRunFunc := range service.preRunFuncs {
		if err := preRunFunc(ctx, preparedQuery, preparedArgs); err != nil {
			return nil, err
		}
	}

	result, err := service.standardLibraryDB.Exec(preparedQuery, preparedArgs...)
	if err != nil {
		return nil, err
	}

	for _, postRunFunc := range service.postRunFuncs {
		if err := postRunFunc(ctx); err != nil {
			return nil, err
		}
	}

	return result, nil
}

func shouldBeJson(fieldDefinition reflect.StructField) bool {
	// JSON encode slices
	if fieldDefinition.Type.Kind() == reflect.Slice {
		return true
	}

	// JSON encode structs
	if fieldDefinition.Type.Kind() == reflect.Struct {
		// Don't JSON encode time.Time
		if reflect.TypeFor[time.Time]() == fieldDefinition.Type {
			return false
		}

		return true
	}

	return false
}
