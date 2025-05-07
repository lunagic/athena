package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	_ "github.com/lib/pq"
	"github.com/lunagic/athena/athenaservices/database/internal/utils"
)

func NewDriverPostgres(config DriverPostgresConfig) Driver {
	return &driverPostgres{
		config: config,
	}
}

type DriverPostgresConfig struct {
	Host string
	Port int
	User string
	Pass string
	Name string
}

type driverPostgres struct {
	config  DriverPostgresConfig
	mapping map[uintptr]string
}

func (driver *driverPostgres) Open() (*sql.DB, error) {
	return sql.Open(
		"postgres",
		fmt.Sprintf(
			"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			driver.config.Host,
			driver.config.Port,
			driver.config.User,
			driver.config.Pass,
			driver.config.Name,
		),
	)
}

func (driver *driverPostgres) setMapping(mapping map[uintptr]string) {
	driver.mapping = mapping
}

func (driver *driverPostgres) autoMigrateAdjustTableDefinition(table Table) Table {
	return table
}

func (driver *driverPostgres) autoMigrateColumnAlter(table Table, column TableColumn) ([]statement, error) {
	statements := []statement{}

	{ // Comments
		statements = append(statements, statement{
			Query: fmt.Sprintf(
				`COMMENT ON COLUMN "%s"."%s" IS '%s';`,
				table.Name,
				column.Name,
				column.Comment,
			),
		})
	}

	{ // Nullable
		if column.Nullable {
			statements = append(statements, statement{
				Query: fmt.Sprintf(
					`ALTER TABLE "%s" ALTER COLUMN "%s" DROP NOT NULL;`,
					table.Name,
					column.Name,
				),
			})
		} else {
			statements = append(statements, statement{
				Query: fmt.Sprintf(
					`ALTER TABLE "%s" ALTER COLUMN "%s" SET NOT NULL;`,
					table.Name,
					column.Name,
				),
			})
		}
	}

	{ // Confirm type
		statements = append(statements, statement{
			Query: fmt.Sprintf(
				`ALTER TABLE "%s" ALTER COLUMN "%s" TYPE %s;`,
				table.Name,
				column.Name,
				column.Type,
			),
		})
	}

	{ // Defaults
		if column.Default == nil {
			statements = append(statements, statement{
				Query: fmt.Sprintf(
					`ALTER TABLE "%s" ALTER COLUMN "%s" DROP DEFAULT;`,
					table.Name,
					column.Name,
				),
			})
		} else {
			statements = append(statements, statement{
				Query: fmt.Sprintf(
					`ALTER TABLE "%s" ALTER COLUMN "%s" SET DEFAULT %s;`,
					table.Name,
					column.Name,
					*column.Default,
				),
			})
		}
	}

	return statements, nil
}

func (driver *driverPostgres) autoMigrateColumnCreate(table Table, column TableColumn) ([]statement, error) {
	return []statement{
		{
			Query: fmt.Sprintf(
				`ALTER TABLE "%s" ADD COLUMN %s;`,
				table.Name,
				driver.renderColumn(column),
			),
			Parameters: map[string]any{},
		},
	}, nil
}

func (driver *driverPostgres) autoMigrateColumnDrop(table Table, column TableColumn) ([]statement, error) {
	return []statement{
		{
			Query: fmt.Sprintf(
				`ALTER TABLE "%s" DROP COLUMN "%s";`,
				table.Name,
				column.Name,
			),
			Parameters: map[string]any{},
		},
	}, nil
}

func (driver *driverPostgres) autoMigrateIndexAlter(table Table, index TableIndex) ([]statement, error) {
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

func (driver *driverPostgres) autoMigrateIndexCreate(table Table, index TableIndex) ([]statement, error) {
	unique := ""
	if index.Unique {
		unique = " UNIQUE"
	}

	return []statement{
		{
			Query: fmt.Sprintf(
				`CREATE%s INDEX "%s" ON "%s" (%s)`,
				unique,
				index.Name,
				table.Name,
				strings.Join(index.Columns, ", "),
			),
			Parameters: map[string]any{},
		},
	}, nil
}

func (driver *driverPostgres) autoMigrateIndexDrop(table Table, index TableIndex) ([]statement, error) {

	return []statement{
		{
			Query: fmt.Sprintf(
				`DROP INDEX "%s";`,
				index.Name,
			),
			Parameters: map[string]any{},
		},
	}, nil
}

func (driver *driverPostgres) autoMigrateOverride(sourceTable Table, targetTable Table) []statement {
	return nil
}

func (driver *driverPostgres) autoMigrateTableCreate(table Table) ([]statement, error) {
	parts := []string{}
	for _, column := range table.columns {

		parts = append(parts, driver.renderColumn(column))
	}

	for _, column := range table.columns {
		if column.ForeignKey.TargetTable == "" {
			continue
		}
		parts = append(parts, driver.renderForeignKey(table, column))
	}

	statements := []statement{
		{
			Query: fmt.Sprintf(
				`CREATE TABLE  "%s" (%s)`,
				table.Name,
				strings.Join(parts, ", "),
			),
			Parameters: map[string]any{},
		},
	}

	for _, column := range table.columns {
		if column.Comment == "" {
			continue
		}
		statements = append(statements, statement{
			Query: fmt.Sprintf(
				`COMMENT ON COLUMN "%s"."%s" IS '%s';`,
				table.Name,
				column.Name,
				column.Comment,
			),
		})
	}

	for _, index := range table.Indexes {
		i, err := driver.autoMigrateIndexCreate(table, index)
		if err != nil {
			return nil, err
		}
		statements = append(statements, i...)
	}

	return statements, nil
}

func (driver *driverPostgres) autoMigrateTableDrop(table Table) ([]statement, error) {
	return []statement{
		{
			Query: fmt.Sprintf(
				`DROP TABLE "%s";`,
				table.Name,
			),
			Parameters: map[string]any{},
		},
	}, nil
}

func (driver *driverPostgres) autoMigrateTableGet(ctx context.Context, service *Service, tableName string) (Table, error) {
	table := Table{
		Name: tableName,
	}
	columns := []postgresColumn{}
	if err := service.runSelect(ctx, statement{
		Query: fmt.Sprintf(`
			SELECT
				tableColumns.column_name,
				tableColumns.data_type,
				tableColumns.identity_generation,
				tableColumns.is_nullable,
				tableColumns.column_default,
				primaryKeys.constraint_type as primary_key,
				descriptions.description
			FROM
				information_schema.columns AS tableColumns
			LEFT JOIN pg_catalog.pg_statio_all_tables allTables ON (
				allTables.relname = tableColumns.table_name
			)
			LEFT JOIN pg_catalog.pg_description descriptions ON (
				allTables.relid = descriptions.objoid
				AND tableColumns.ordinal_position = descriptions.objsubid
			)
			LEFT JOIN (
				SELECT tc.table_name, kcu.column_name,tc.constraint_type
					FROM information_schema.table_constraints tc
					JOIN information_schema.key_column_usage kcu
					ON tc.constraint_name = kcu.constraint_name
					AND tc.table_schema = kcu.table_schema
					AND tc.constraint_type = 'PRIMARY KEY'
					AND tc.table_catalog = '%s'
			) primaryKeys ON (
				primaryKeys.table_name = tableColumns.table_name
				AND primaryKeys.column_name = tableColumns.column_name
			)
			WHERE
				tableColumns.table_name = '%s';
		`, driver.config.Name, tableName),
	}, &columns); err != nil {
		return Table{}, err
	}

	if len(columns) == 0 {
		return Table{}, ErrTableNotFound
	}

	foreignKeys := map[string]tableForeignKey{}
	postgresForeignKeys := []postgresForeignKey{}
	if err := service.runSelect(ctx, statement{
		Query: fmt.Sprintf(
			`
				SELECT
					conname AS constraint_name,
					att2.attname AS column_name,
					cl.relname AS referenced_table,
					att.attname AS referenced_column
				FROM
					(SELECT
						unnest(con1.conkey) AS parent,
						unnest(con1.confkey) AS referenced,
						con1.conname,
						con1.confrelid,
						con1.conrelid
					FROM pg_class cl
					JOIN pg_constraint con1 ON con1.conrelid = cl.oid
					WHERE con1.contype = 'f'
					AND cl.relname = '%s') con
				JOIN pg_attribute att ON att.attnum = con.referenced
					AND att.attrelid = con.confrelid
				JOIN pg_class cl ON cl.oid = con.confrelid
				JOIN pg_attribute att2 ON att2.attnum = con.parent
					AND att2.attrelid = con.conrelid;
			`,
			tableName,
		),
	}, &postgresForeignKeys); err != nil {
		return Table{}, err
	}

	for _, foreignKey := range postgresForeignKeys {
		foreignKeys[foreignKey.Column] = tableForeignKey{
			TargetTable:  foreignKey.ReferencedTable,
			TargetColumn: foreignKey.ReferencedColumn,
		}
	}

	for _, column := range columns {
		nullable := column.Nullable == "YES"
		defaultValue := column.Default
		if nullable && defaultValue == nil {
			x := "NULL"
			defaultValue = &x
		}

		table.columns = append(table.columns, TableColumn{
			Name:          column.Name,
			Type:          column.Type,
			Default:       defaultValue,
			AutoIncrement: column.IdentityGeneration != nil && *column.IdentityGeneration == "ALWAYS",
			PrimaryKey:    column.PrimaryKey != nil,
			Nullable:      nullable,
			ForeignKey:    foreignKeys[column.Name],
			Comment: func() string {
				if column.Comment == nil {
					return ""
				}

				return *column.Comment
			}(),
		})
	}

	indexes := []postgresIndex{}
	if err := service.runSelect(ctx, statement{
		Query: fmt.Sprintf(`
			SELECT
				i.relname AS index_name,
				STRING_AGG(a.attname, ',') AS indexed_columns,
				ix.indisunique AS is_unique
			FROM
				pg_index ix
			JOIN
				pg_class i ON i.oid = ix.indexrelid
			JOIN
				pg_class t ON t.oid = ix.indrelid
			JOIN
				pg_attribute a ON a.attnum = ANY(ix.indkey) AND a.attrelid = t.oid
			WHERE
				t.relname = '%s'
				AND i.relname NOT LIKE '%%_pkey'
			GROUP BY
				i.relname, ix.indisunique;
		`, tableName),
	}, &indexes); err != nil {
		return Table{}, err
	}

	for _, index := range indexes {
		table.Indexes = append(table.Indexes, TableIndex{
			Name:    index.Name,
			Columns: strings.Split(index.Columns, ","),
			Unique:  index.Unique,
		})
	}

	return table, nil
}

func (driver *driverPostgres) convertTypeBool() string {
	return "boolean"
}

func (driver *driverPostgres) convertTypeInt() string {
	return "bigint"
}

func (driver *driverPostgres) convertTypeInt8() string {
	return "smallint"
}

func (driver *driverPostgres) convertTypeInt16() string {
	return "smallint"
}

func (driver *driverPostgres) convertTypeInt32() string {
	return "integer"
}

func (driver *driverPostgres) convertTypeInt64() string {
	return "bigint"
}

func (driver *driverPostgres) convertTypeUint() string {
	return "bigint"
}

func (driver *driverPostgres) convertTypeUint8() string {
	return "smallint"
}

func (driver *driverPostgres) convertTypeUint16() string {
	return "smallint"
}

func (driver *driverPostgres) convertTypeUint32() string {
	return "integer"
}

func (driver *driverPostgres) convertTypeUint64() string {
	return "bigint"
}

func (driver *driverPostgres) convertTypeFloat32() string {
	return "real"
}

func (driver *driverPostgres) convertTypeFloat64() string {
	return "double precision"
}

func (driver *driverPostgres) convertTypeString() string {
	return "text"
}

func (driver *driverPostgres) convertTypeDateTime() string {
	return "timestamp without time zone"
}

func (driver *driverPostgres) convertTypeJSON() string {
	return "json"
}

func (driver *driverPostgres) generateDelete(e Entity) (statement, error) {
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
			`DELETE FROM "%s" WHERE id = %d`,
			e.TableStructure().Name,
			id,
		),
		Parameters: map[string]any{},
	}, nil
}

func (driver *driverPostgres) generateInsert(e Entity) (statement, error) {
	columns := []string{}
	values := []string{}
	parameters := map[string]any{}
	primaryKey := ""

	if err := utils.LoopOverStructFields(reflect.ValueOf(e), func(fieldDefinition reflect.StructField, fieldValue reflect.Value) error {
		tag := utils.ParseTag(fieldDefinition.Tag)
		if tag.Column == "" {
			return nil
		}

		if tag.ReadOnly {
			return nil
		}

		if tag.PrimaryKey {
			primaryKey = tag.Column
		}

		if tag.AutoIncrement {
			return nil
		}

		column := fmt.Sprintf(`"%s"`, tag.Column)
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
			`INSERT INTO "%s" (%s) VALUES (%s) RETURNING "%s" as "id"`,
			e.TableStructure().Name,
			strings.Join(columns, ", "),
			strings.Join(values, ", "),
			primaryKey,
		),
		Parameters: parameters,
	}, nil
}

func (driver *driverPostgres) generateSelect(query Query) (statement, error) {
	selects := []string{}
	for _, column := range query.Select {
		selects = append(selects, fmt.Sprintf(`"%s"`, column))
	}

	queryString := fmt.Sprintf(`SELECT %s FROM "%s"`, strings.Join(selects, ", "), query.From)

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

func (driver *driverPostgres) generateSimpleOperatorOfEquality(o simpleOperatorOfEquality) (statement, error) {
	columnName := driver.mapping[uintptr(reflect.ValueOf(o.Column).UnsafePointer())]
	if columnName == "" {
		return statement{}, errors.New("unknown column")
	}

	key := fmt.Sprintf(":%s", columnName)

	return statement{
		Query: fmt.Sprintf(`"%s" %s %s`, columnName, o.Operator, key),
		Parameters: map[string]any{
			key: o.Value,
		},
	}, nil
}

func (driver *driverPostgres) generateSimpleOperatorOfLogic(o simpleOperatorOfLogic) (statement, error) {
	return generateSimpleOperatorOfLogic(driver, o)
}

func (driver *driverPostgres) generateUpdate(e Entity) (statement, error) {
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
		sets = append(sets, fmt.Sprintf(`"%s" = %s`, tag.Column, value))

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
			`UPDATE "%s" SET %s WHERE id = %d`,
			e.TableStructure().Name,
			strings.Join(sets, ", "),
			id,
		),
		Parameters: parameters,
	}, nil
}

func (driver *driverPostgres) usesLastInsertId() bool {
	return false
}

func (driver *driverPostgres) usesNumberedParameters() bool {
	return true
}

type postgresColumn struct {
	Name               string  `db:"column_name"`
	Type               string  `db:"data_type"`
	IdentityGeneration *string `db:"identity_generation"`
	PrimaryKey         *string `db:"primary_key"`
	Nullable           string  `db:"is_nullable"`
	Default            *string `db:"column_default"`
	Comment            *string `db:"description"`
}

type postgresIndex struct {
	Name    string `db:"index_name"`
	Columns string `db:"indexed_columns"`
	Unique  bool   `db:"is_unique"`
}

type postgresForeignKey struct {
	Name             string `db:"constraint_name"`
	Column           string `db:"column_name"`
	ReferencedTable  string `db:"referenced_table"`
	ReferencedColumn string `db:"referenced_column"`
}

func (driver *driverPostgres) renderColumn(column TableColumn) string {
	nullable := ""
	if !column.Nullable {
		nullable = " NOT NULL"
	}

	columnDefault := ""
	if column.Default != nil {
		columnDefault = fmt.Sprintf(" DEFAULT %s", *column.Default)
	}

	extras := ""
	if column.PrimaryKey {
		extras = " PRIMARY KEY"
		if column.AutoIncrement {
			extras = " GENERATED ALWAYS AS IDENTITY PRIMARY KEY"
		}
	}

	return fmt.Sprintf(
		`"%s" %s%s%s%s`,
		column.Name,
		column.Type,
		extras,
		nullable,
		columnDefault,
	)
}

func (driver *driverPostgres) renderForeignKey(table Table, column TableColumn) string {
	return fmt.Sprintf(
		`CONSTRAINT "%s" FOREIGN KEY ("%s") REFERENCES "%s"("%s") ON DELETE CASCADE`,
		fmt.Sprintf(
			"fk_%s_%s_%s_%s",
			table.Name,
			column.Name,
			column.ForeignKey.TargetTable,
			column.ForeignKey.TargetColumn,
		),
		column.Name,
		column.ForeignKey.TargetTable,
		column.ForeignKey.TargetColumn,
	)
}
