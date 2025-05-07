package database

import (
	"fmt"
	"reflect"
	"time"

	"github.com/lunagic/athena/athenaservices/database/internal/utils"
)

type ErrUnsupportedType struct {
	Type string
}

func (err ErrUnsupportedType) Error() string {
	return fmt.Sprintf("unsupported type: %s", err.Type)
}

type Entity interface {
	TableStructure() Table
}

type Table struct {
	Name    string
	Comment string
	columns []TableColumn
	Indexes []TableIndex
}

type tableLookups struct {
	columns map[string]TableColumn
	indexes map[string]TableIndex
}

func getMapping(driver Driver) map[any]func() string {
	mapping := map[any]func() string{
		reflect.Bool:               driver.convertTypeBool,
		reflect.Int:                driver.convertTypeInt,
		reflect.Int8:               driver.convertTypeInt8,
		reflect.Int16:              driver.convertTypeInt16,
		reflect.Int32:              driver.convertTypeInt32,
		reflect.Int64:              driver.convertTypeInt64,
		reflect.Uint:               driver.convertTypeUint,
		reflect.Uint8:              driver.convertTypeUint8,
		reflect.Uint16:             driver.convertTypeUint16,
		reflect.Uint32:             driver.convertTypeUint32,
		reflect.Uint64:             driver.convertTypeUint64,
		reflect.Float32:            driver.convertTypeFloat32,
		reflect.Float64:            driver.convertTypeFloat64,
		reflect.String:             driver.convertTypeString,
		reflect.Slice:              driver.convertTypeJSON,
		reflect.Struct:             driver.convertTypeJSON,
		reflect.TypeOf(time.Now()): driver.convertTypeDateTime,
	}

	return mapping
}

func translateTypeFromService(driver Driver, t reflect.Type) (string, error) {
	mapping := getMapping(driver)

	{ // First look for exact matches (time.Time)
		match, found := mapping[t]
		if found {
			return match(), nil
		}
	}

	{ // Then look for more generic kind matches
		match, found := mapping[t.Kind()]
		if found {
			return match(), nil
		}
	}

	return "", ErrUnsupportedType{
		Type: t.String(),
	}
}

func fieldToType(driver Driver, field reflect.StructField) (TableColumn, error) {
	tag := utils.ParseTag(field.Tag)

	comment := tag.Comment

	column := TableColumn{
		Name:    tag.Column,
		Comment: comment,
		ForeignKey: tableForeignKey{
			TargetTable:  tag.ForeignKeyTargetTable,
			TargetColumn: tag.ForeignKeyTargetColumn,
		},
		PrimaryKey:    tag.PrimaryKey,
		AutoIncrement: tag.AutoIncrement,
		Default: func() *string {
			if tag.Default == "" {
				return nil
			}

			return &tag.Default
		}(),
	}

	fieldType := field.Type
	if field.Type.Kind() == reflect.Pointer {
		column.Nullable = true
		column.Default = func(s string) *string {
			return &s
		}("NULL")
		fieldType = fieldType.Elem()
	}

	columnType, err := translateTypeFromService(driver, fieldType)
	if err != nil {
		return TableColumn{}, err
	}
	column.Type = columnType

	return column, nil
}

func (table *Table) hydrateColumns(driver Driver, entity Entity) error {
	typeOf := reflect.TypeOf(entity)
	columns := []TableColumn{}
	for i := range typeOf.NumField() {
		field := typeOf.Field(i)

		column, err := fieldToType(driver, field)
		if err != nil {
			return err
		}
		if column.Name == "" {
			continue
		}

		columns = append(columns, column)
	}

	table.columns = columns

	return nil
}

func (table Table) lookups() tableLookups {
	lookup := tableLookups{
		columns: map[string]TableColumn{},
		indexes: map[string]TableIndex{},
	}

	for _, index := range table.Indexes {
		lookup.indexes[index.Name] = index
	}

	for _, column := range table.columns {
		lookup.columns[column.Name] = column
	}

	return lookup
}

type TableColumn struct {
	Name          string
	Type          string
	Default       *string
	Nullable      bool
	Comment       string
	PrimaryKey    bool
	AutoIncrement bool
	ForeignKey    tableForeignKey
}

type TableIndex struct {
	Name    string
	Columns []string
	Unique  bool
}

type tableForeignKey struct {
	// Name         string
	TargetTable  string
	TargetColumn string
}
