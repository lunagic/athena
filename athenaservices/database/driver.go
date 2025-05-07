package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"strings"
)

var (
	ErrNoRows                   = errors.New("no rows found")
	ErrBlankQuery               = errors.New("blank query")
	ErrTableNotFound            = errors.New("table not found")
	errNeedsAutoMigrateOverride = errors.New("needs auto migrate override")
)

type Driver interface {
	Open() (*sql.DB, error)
	setMapping(mapping map[uintptr]string)
	autoMigrateAdjustTableDefinition(table Table) Table
	autoMigrateColumnAlter(table Table, column TableColumn) ([]statement, error)
	autoMigrateColumnCreate(table Table, column TableColumn) ([]statement, error)
	autoMigrateColumnDrop(table Table, column TableColumn) ([]statement, error)
	autoMigrateIndexAlter(table Table, column TableIndex) ([]statement, error)
	autoMigrateIndexCreate(table Table, column TableIndex) ([]statement, error)
	autoMigrateIndexDrop(table Table, column TableIndex) ([]statement, error)
	autoMigrateOverride(sourceTable Table, targetTable Table) []statement
	autoMigrateTableCreate(table Table) ([]statement, error)
	autoMigrateTableDrop(table Table) ([]statement, error)
	autoMigrateTableGet(ctx context.Context, service *Service, tableName string) (Table, error)
	convertTypeBool() string
	convertTypeDateTime() string
	convertTypeFloat32() string
	convertTypeFloat64() string
	convertTypeInt() string
	convertTypeInt16() string
	convertTypeInt32() string
	convertTypeInt64() string
	convertTypeInt8() string
	convertTypeJSON() string
	convertTypeString() string
	convertTypeUint() string
	convertTypeUint16() string
	convertTypeUint32() string
	convertTypeUint64() string
	convertTypeUint8() string
	generateDelete(entity Entity) (statement, error)
	generateInsert(entity Entity) (statement, error)
	generateSelect(query Query) (statement, error)
	generateSimpleOperatorOfEquality(o simpleOperatorOfEquality) (statement, error)
	generateSimpleOperatorOfLogic(o simpleOperatorOfLogic) (statement, error)
	generateUpdate(entity Entity) (statement, error)
	usesLastInsertId() bool
	usesNumberedParameters() bool
}

func generateSimpleOperatorOfLogic(driver Driver, o simpleOperatorOfLogic) (statement, error) {
	parts := []string{}
	parameters := map[string]any{}

	for _, x := range o.operatorsEvaluation {
		subStatement, err := x.haveDriverRender(driver)
		if err != nil {
			return subStatement, err
		}

		parts = append(parts, subStatement.Query)
		maps.Copy(parameters, subStatement.Parameters) // TODO: throw error for overlap of paramKey
	}

	return statement{
		Query:      fmt.Sprintf("(%s)", strings.Join(parts, " "+o.operatorKeyword+" ")),
		Parameters: parameters,
	}, nil
}
