package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/google/uuid"
	"github.com/lunagic/athena/athenaservices/database/internal/utils"
	_ "github.com/mattn/go-sqlite3"
)

func NewDriverSQLite(path string) Driver {
	return &driverSQLite{
		Path: path,
	}
}

type driverSQLite struct {
	Path    string
	mapping map[uintptr]string
}

func (driver *driverSQLite) Open() (*sql.DB, error) {
	return sql.Open(
		"sqlite3",
		fmt.Sprintf("file:%s?cache=shared&_foreign_keys=on", driver.Path),
	)
}

func (driver *driverSQLite) setMapping(mapping map[uintptr]string) {
	driver.mapping = mapping
}

func (driver *driverSQLite) autoMigrateAdjustTableDefinition(table Table) Table {
	// Does not support comments
	table.Comment = ""
	for i, column := range table.columns {
		column.Comment = ""
		table.columns[i] = column
	}

	return table
}

func (driver *driverSQLite) autoMigrateColumnAlter(table Table, column TableColumn) ([]statement, error) {
	return nil, errNeedsAutoMigrateOverride
}

func (driver *driverSQLite) autoMigrateColumnCreate(table Table, column TableColumn) ([]statement, error) {
	return []statement{
		{
			Query: fmt.Sprintf(
				`ALTER TABLE "%s" ADD COLUMN %s`,
				table.Name,
				driver.renderColumn(column),
			),
		},
	}, nil
}

func (driver *driverSQLite) autoMigrateColumnDrop(table Table, column TableColumn) ([]statement, error) {
	return []statement{
		{
			Query: fmt.Sprintf(
				`ALTER TABLE "%s" DROP COLUMN %s`,
				table.Name,
				column.Name,
			),
		},
	}, nil
}

func (driver *driverSQLite) autoMigrateIndexAlter(table Table, index TableIndex) ([]statement, error) {
	drop, err := driver.autoMigrateIndexDrop(table, index)
	if err != nil {
		return nil, err
	}

	create, err := driver.autoMigrateIndexCreate(table, index)
	if err != nil {
		return nil, err
	}

	return append(drop, create...), nil
}

func (driver *driverSQLite) autoMigrateIndexCreate(table Table, index TableIndex) ([]statement, error) {
	return []statement{
		{
			Query: fmt.Sprintf(
				`CREATE%s INDEX "%s" ON "%s" (%s)`,
				func() string {
					if index.Unique {
						return " UNIQUE"
					}

					return ""
				}(),
				index.Name,
				table.Name,
				strings.Join(index.Columns, ", "),
			),
		},
	}, nil
}

func (driver *driverSQLite) autoMigrateIndexDrop(table Table, index TableIndex) ([]statement, error) {
	return []statement{
		{
			Query: fmt.Sprintf(
				`DROP INDEX IF EXISTS %s`,
				index.Name,
			),
		},
	}, nil
}

func (driver *driverSQLite) autoMigrateOverride(sourceTable Table, targetTable Table) []statement {
	tempTable := targetTable
	tempTable.Name = uuid.NewString()

	statements := []statement{}

	for _, index := range sourceTable.Indexes {
		s, err := driver.autoMigrateIndexDrop(sourceTable, index)
		if err != nil {
			return nil
		}
		statements = append(statements, s...)
	}

	crateStatements, err := driver.autoMigrateTableCreate(tempTable)
	if err != nil {
		return nil
	}

	statements = append(statements, crateStatements...)

	columns := strings.Join(func() []string {
		sourceColumns := map[string]bool{}
		for _, column := range sourceTable.columns {
			sourceColumns[column.Name] = true
		}

		columns := []string{}
		for _, column := range targetTable.columns {
			if _, ok := sourceColumns[column.Name]; !ok {
				continue
			}

			columns = append(columns, column.Name)
		}

		return columns
	}(), ", ")

	// Copy data over from old table to new table
	statements = append(statements, statement{
		Query: fmt.Sprintf(
			`INSERT INTO "%s" (%s) SELECT %s FROM %s`,
			tempTable.Name,
			columns,
			columns,
			targetTable.Name,
		),
	})

	// Drop original table
	statements = append(statements, statement{
		Query: fmt.Sprintf(
			`DROP TABLE "%s"`,
			targetTable.Name,
		),
	})

	// Rename temp table to original name
	statements = append(statements, statement{
		Query: fmt.Sprintf(
			`ALTER TABLE "%s" RENAME TO "%s";`,
			tempTable.Name,
			targetTable.Name,
		),
	})

	return statements
}

func (driver *driverSQLite) autoMigrateTableCreate(table Table) ([]statement, error) {
	parts := []string{}
	for _, column := range table.columns {
		parts = append(parts, driver.renderColumn(column))
	}

	statements := []statement{{
		Query: fmt.Sprintf(
			`CREATE TABLE "%s" (%s)`,
			table.Name,
			strings.Join(parts, ", "),
		),
	}}

	for _, index := range table.Indexes {
		indexStatements, err := driver.autoMigrateIndexCreate(table, index)
		if err != nil {
			return nil, err
		}

		statements = append(statements, indexStatements...)
	}

	return statements, nil
}

func (driver *driverSQLite) autoMigrateTableDrop(table Table) ([]statement, error) {
	return nil, nil
}

func (driver *driverSQLite) autoMigrateTableGet(ctx context.Context, service *Service, tableName string) (Table, error) {
	table := Table{
		Name: tableName,
	}

	tables := []sqliteTableStruct{}
	if err := service.runSelect(ctx, statement{
		Query: `
			SELECT sql
			FROM sqlite_master
			WHERE name = :tableName;
		`,
		Parameters: map[string]any{
			":tableName": tableName,
		},
	}, &tables); err != nil {
		return Table{}, err
	}

	if len(tables) == 0 {
		return Table{}, ErrTableNotFound
	}

	autoIncrementing := strings.Contains(tables[0].SQL, "PRIMARY KEY AUTOINCREMENT")

	foreignKeys := map[string]tableForeignKey{}
	sqliteForeignKeys := []sqliteForeignKey{}
	if err := service.runSelect(ctx, statement{
		Query: fmt.Sprintf(`PRAGMA foreign_key_list("%s");`, tableName),
	}, &sqliteForeignKeys); err != nil {
		return Table{}, err
	}

	for _, foreignKey := range sqliteForeignKeys {
		foreignKeys[foreignKey.From] = tableForeignKey{
			TargetTable:  foreignKey.Table,
			TargetColumn: foreignKey.To,
		}
	}

	columns := []sqliteTableInfo{}
	if err := service.runSelect(ctx, statement{
		Query: `
			SELECT
				name AS column_name,
				TYPE AS column_type,
				pk as primary_key,
				"notnull" as not_null,
				dflt_value AS column_default
			FROM
				pragma_table_xinfo(:tableName)
		`,
		Parameters: map[string]any{
			":tableName": tableName,
		},
	}, &columns); err != nil {
		return Table{}, err
	}

	for _, column := range columns {
		nullable := !column.NotNull
		defaultValue := column.ColumnDefault
		if nullable && defaultValue == nil {
			x := "NULL"
			defaultValue = &x
		}

		table.columns = append(table.columns, TableColumn{
			Name:          column.ColumnName,
			Type:          column.ColumnType,
			PrimaryKey:    column.PrimaryKey,
			AutoIncrement: autoIncrementing && column.PrimaryKey,
			Default:       defaultValue,
			Nullable:      nullable,
			ForeignKey:    foreignKeys[column.ColumnName],
		})
	}

	indexes := []sqliteIndex{}
	if err := service.runSelect(ctx, statement{
		Query: `
			SELECT
				il.name AS index_name,
				il."unique",
				ii.name AS 'column_name'
			FROM
				sqlite_master,
				pragma_index_list (sqlite_master.name) AS il,
				pragma_index_info (il.name) AS ii
			WHERE
				sqlite_master.name = :tableName;
		`,
		Parameters: map[string]any{
			":tableName": tableName,
		},
	}, &indexes); err != nil {
		return Table{}, err
	}

	lookupForDuplicates := map[string]int{}

	for _, index := range indexes {
		existingIndexIndex, alreadySeenThisKey := lookupForDuplicates[index.Name]
		if alreadySeenThisKey {
			table.Indexes[existingIndexIndex].Columns = append(table.Indexes[existingIndexIndex].Columns, index.Column)
			continue
		}

		// Skip these auto-indexes for the primary key since we didn't create them and we don't want to try and remove them
		if strings.HasPrefix(index.Name, "sqlite_autoindex") {
			continue
		}

		table.Indexes = append(table.Indexes, TableIndex{
			Name:    index.Name,
			Columns: []string{index.Column},
			Unique:  index.Unique,
		})
		lookupForDuplicates[index.Name] = len(table.Indexes) - 1
	}

	return table, nil
}

func (driver *driverSQLite) convertTypeBool() string {
	return "INTEGER"
}

func (driver *driverSQLite) convertTypeInt() string {
	return "INTEGER"
}

func (driver *driverSQLite) convertTypeInt8() string {
	return "INTEGER"
}

func (driver *driverSQLite) convertTypeInt16() string {
	return "INTEGER"
}

func (driver *driverSQLite) convertTypeInt32() string {
	return "INTEGER"
}

func (driver *driverSQLite) convertTypeInt64() string {
	return "INTEGER"
}

func (driver *driverSQLite) convertTypeUint() string {
	return "INTEGER UNSIGNED"
}

func (driver *driverSQLite) convertTypeUint8() string {
	return "INTEGER"
}

func (driver *driverSQLite) convertTypeUint16() string {
	return "INTEGER"
}

func (driver *driverSQLite) convertTypeUint32() string {
	return "INTEGER"
}

func (driver *driverSQLite) convertTypeUint64() string {
	return "INTEGER"
}

func (driver *driverSQLite) convertTypeFloat32() string {
	return "INTEGER"
}

func (driver *driverSQLite) convertTypeFloat64() string {
	return "INTEGER"
}

func (driver *driverSQLite) convertTypeString() string {
	return "TEXT"
}

func (driver *driverSQLite) convertTypeDateTime() string {
	return "DATETIME"
}

func (driver *driverSQLite) convertTypeJSON() string {
	return "TEXT"
}

func (driver *driverSQLite) generateDelete(e Entity) (statement, error) {
	id := int64(0)

	if err := utils.LoopOverStructFields(reflect.ValueOf(e), func(fieldDefinition reflect.StructField, fieldValue reflect.Value) error {
		tag := utils.ParseTag(fieldDefinition.Tag)
		if tag.Column == "" {
			return nil
		}

		if tag.ReadOnly {
			return nil
		}

		if tag.PrimaryKey {
			id = fieldValue.Int()
			return nil
		}

		return nil
	}); err != nil {
		return statement{}, err
	}

	return statement{
		Query: fmt.Sprintf(
			"DELETE FROM `%s` WHERE id = %d",
			e.TableStructure().Name,
			id,
		),
		Parameters: map[string]any{},
	}, nil
}

func (driver *driverSQLite) generateInsert(e Entity) (statement, error) {
	columns := []string{}
	values := []string{}
	parameters := map[string]any{}

	if err := utils.LoopOverStructFields(reflect.ValueOf(e), func(fieldDefinition reflect.StructField, fieldValue reflect.Value) error {
		tag := utils.ParseTag(fieldDefinition.Tag)
		if tag.Column == "" {
			return nil
		}

		if tag.ReadOnly {
			return nil
		}

		if tag.AutoIncrement {
			return nil
		}

		column := fmt.Sprintf("`%s`", tag.Column)
		value := fmt.Sprintf(":%s", tag.Column)

		columns = append(columns, column)
		values = append(values, value)

		if shouldBeJson(fieldDefinition) {
			fieldBytes, err := json.Marshal(fieldValue.Interface())
			if err != nil {
				return err
			}

			parameters[value] = string(fieldBytes)
		} else {
			parameters[value] = fieldValue.Interface()
		}

		return nil
	}); err != nil {
		return statement{}, err
	}

	return statement{
		Query: fmt.Sprintf(
			"INSERT INTO `%s` (%s) VALUES (%s)",
			e.TableStructure().Name,
			strings.Join(columns, ", "),
			strings.Join(values, ", "),
		),
		Parameters: parameters,
	}, nil
}

func (driver *driverSQLite) generateSelect(query Query) (statement, error) {
	selects := []string{}
	for _, column := range query.Select {
		selects = append(selects, fmt.Sprintf("`%s`", column))
	}

	queryString := fmt.Sprintf("SELECT %s FROM `%s`", strings.Join(selects, ", "), query.From)

	parameters := map[string]any{}
	if query.Where != nil && query.Where.hasAny() {
		s, err := query.Where.haveDriverRender(driver)
		if err != nil {
			return statement{}, err
		}
		queryString += " WHERE " + s.Query
		for k, v := range s.Parameters {
			parameters[k] = v
		}
	}

	return statement{
		Query:      queryString,
		Parameters: parameters,
	}, nil
}

func (driver *driverSQLite) generateSimpleOperatorOfEquality(o simpleOperatorOfEquality) (statement, error) {
	columnName := driver.mapping[uintptr(reflect.ValueOf(o.Column).UnsafePointer())]
	if columnName == "" {
		return statement{}, errors.New("unknown column")
	}

	key := fmt.Sprintf(":%s", columnName)

	return statement{
		Query: fmt.Sprintf("`%s` %s %s", columnName, o.Operator, key),
		Parameters: map[string]any{
			key: o.Value,
		},
	}, nil
}

func (driver *driverSQLite) generateSimpleOperatorOfLogic(o simpleOperatorOfLogic) (statement, error) {
	return generateSimpleOperatorOfLogic(driver, o)
}

func (driver *driverSQLite) generateUpdate(e Entity) (statement, error) {
	sets := []string{}
	id := int64(0)
	parameters := map[string]any{}

	if err := utils.LoopOverStructFields(reflect.ValueOf(e), func(fieldDefinition reflect.StructField, fieldValue reflect.Value) error {
		tag := utils.ParseTag(fieldDefinition.Tag)
		if tag.Column == "" {
			return nil
		}

		if tag.ReadOnly {
			return nil
		}

		if tag.PrimaryKey {
			id = fieldValue.Int()
			return nil
		}

		value := fmt.Sprintf(":%s", tag.Column)
		sets = append(sets, fmt.Sprintf("`%s` = %s", tag.Column, value))

		if shouldBeJson(fieldDefinition) {
			fieldBytes, err := json.Marshal(fieldValue.Interface())
			if err != nil {
				return err
			}

			parameters[value] = string(fieldBytes)
		} else {
			parameters[value] = fieldValue.Interface()
		}

		return nil
	}); err != nil {
		return statement{}, err
	}

	return statement{
		Query: fmt.Sprintf(
			"UPDATE `%s` SET %s WHERE id = %d",
			e.TableStructure().Name,
			strings.Join(sets, ", "),
			id,
		),
		Parameters: parameters,
	}, nil
}

func (driver *driverSQLite) usesLastInsertId() bool {
	return true
}

func (driver *driverSQLite) usesNumberedParameters() bool {
	return false
}

type sqliteTableStruct struct {
	SQL string `db:"sql"`
}

type sqliteTableInfo struct {
	ColumnName    string  `db:"column_name"`
	ColumnType    string  `db:"column_type"`
	PrimaryKey    bool    `db:"primary_key"`
	ColumnDefault *string `db:"column_default"`
	NotNull       bool    `db:"not_null"`
}

type sqliteIndex struct {
	Name   string `db:"index_name"`
	Unique bool   `db:"unique"`
	Column string `db:"column_name"`
}

type sqliteForeignKey struct {
	ID       string `db:"id"`
	Seq      string `db:"seq"`
	Table    string `db:"table"`
	From     string `db:"from"`
	To       string `db:"to"`
	OnUpdate string `db:"on_update"`
	OnDelete string `db:"on_delete"`
	Match    string `db:"match"`
}

func (driver *driverSQLite) renderColumn(column TableColumn) string {
	columnType := column.Type

	nullable := " NOT NULL"
	if column.Nullable {
		nullable = ""
	}

	foreignKey := ""
	if column.ForeignKey.TargetTable != "" {
		foreignKey = fmt.Sprintf(
			` REFERENCES "%s"("%s") ON DELETE CASCADE`,
			column.ForeignKey.TargetTable,
			column.ForeignKey.TargetColumn,
		)

	}

	defaultValue := ""
	if column.Default != nil && *column.Default != "" {
		defaultValue = fmt.Sprintf(` DEFAULT %s`, *column.Default)
	}

	primaryKey := ""
	if column.PrimaryKey {
		primaryKey = " PRIMARY KEY"
		if column.AutoIncrement {
			primaryKey += " AUTOINCREMENT"

		}
	}

	return fmt.Sprintf(`"%s" %s%s%s%s%s`, column.Name, columnType, primaryKey, defaultValue, nullable, foreignKey)
}
