package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/lunagic/athena/athenaservices/database/internal/utils"
)

func NewDriverMySQL(config DriverMySQLConfig) Driver {
	return &driverMySQL{
		config: config,
	}
}

type DriverMySQLConfig struct {
	Host string
	Port int
	User string
	Pass string
	Name string
}

type driverMySQL struct {
	config  DriverMySQLConfig
	mapping map[uintptr]string
}

func (driver *driverMySQL) Open() (*sql.DB, error) {
	_ = mysql.SetLogger(log.New(io.Discard, "", log.LstdFlags))

	return sql.Open("mysql", fmt.Sprintf(
		"%s:%s@(%s:%d)/%s?parseTime=true",
		driver.config.User,
		driver.config.Pass,
		driver.config.Host,
		driver.config.Port,
		driver.config.Name,
	))
}

func (driver *driverMySQL) setMapping(mapping map[uintptr]string) {
	driver.mapping = mapping
}

func (driver *driverMySQL) autoMigrateAdjustTableDefinition(table Table) Table {
	return table
}

func (driver *driverMySQL) autoMigrateColumnAlter(table Table, column TableColumn) ([]statement, error) {
	c, err := driver.renderColumn(column)
	if err != nil {
		return nil, err
	}

	return []statement{
		{
			Query: fmt.Sprintf(
				"ALTER TABLE `%s` CHANGE `%s` %s",
				table.Name,
				column.Name,
				c,
			),
		},
	}, nil
}

func (driver *driverMySQL) autoMigrateColumnCreate(table Table, column TableColumn) ([]statement, error) {
	c, err := driver.renderColumn(column)
	if err != nil {
		return nil, err
	}

	return []statement{
		{
			Query: fmt.Sprintf(
				"ALTER TABLE `%s` ADD COLUMN %s",
				table.Name,
				c,
			),
		},
	}, nil
}

func (driver *driverMySQL) autoMigrateColumnDrop(table Table, column TableColumn) ([]statement, error) {
	return []statement{
		{
			Query: fmt.Sprintf(
				"ALTER TABLE `%s` DROP COLUMN `%s`",
				table.Name,
				column.Name,
			),
		},
	}, nil
}

func (driver *driverMySQL) autoMigrateIndexAlter(table Table, index TableIndex) ([]statement, error) {
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

func (driver *driverMySQL) autoMigrateIndexCreate(table Table, index TableIndex) ([]statement, error) {
	indexStatement, err := driver.renderIndex(index)
	if err != nil {
		return nil, err
	}

	return []statement{
		{
			Query: fmt.Sprintf(
				"ALTER TABLE `%s` ADD %s",
				table.Name,
				indexStatement,
			),
		},
	}, nil
}

func (driver *driverMySQL) autoMigrateIndexDrop(table Table, index TableIndex) ([]statement, error) {
	return []statement{
		{
			Query: fmt.Sprintf(
				"ALTER TABLE `%s` DROP INDEX `%s`",
				table.Name,
				index.Name,
			),
		},
	}, nil
}

func (driver *driverMySQL) autoMigrateOverride(sourceTable Table, targetTable Table) []statement {
	return nil
}

func (driver *driverMySQL) autoMigrateTableCreate(table Table) ([]statement, error) {
	parts := []string{}

	for _, column := range table.columns {
		part, err := driver.renderColumn(column)
		if err != nil {
			return nil, err
		}

		parts = append(parts, part)
	}

	for _, index := range table.Indexes {
		part, err := driver.renderIndex(index)
		if err != nil {
			return nil, err
		}

		parts = append(parts, part)
	}

	for _, column := range table.columns {
		if column.ForeignKey.TargetTable == "" {
			continue
		}

		part, err := driver.renderForeignKey(table, column)
		if err != nil {
			return nil, err
		}

		parts = append(parts, part)
	}

	return []statement{
		{
			Query: fmt.Sprintf(
				"CREATE TABLE `%s` (%s) COMMENT='%s'",
				table.Name,
				strings.Join(parts, ", "),
				table.Comment,
			),
		},
	}, nil
}

func (driver *driverMySQL) autoMigrateTableDrop(table Table) ([]statement, error) {
	return []statement{
		{
			Query: fmt.Sprintf("DROP TABLE `%s`", table.Name),
		},
	}, nil
}

func (driver *driverMySQL) autoMigrateTableGet(
	ctx context.Context,
	service *Service,
	tableName string,
) (
	Table,
	error,
) {
	var mysqlMetaData mysqlInfoTableMetaData
	{
		mysqlMetaDataList := []mysqlInfoTableMetaData{}
		if err := service.runSelect(ctx, statement{
			Query: "SHOW TABLE STATUS WHERE Name=:tableName;",
			Parameters: map[string]any{
				":tableName": tableName,
			},
		}, &mysqlMetaDataList); err != nil {
			return Table{}, err
		}

		// table does not exist
		if len(mysqlMetaDataList) == 0 {
			return Table{}, ErrTableNotFound
		}
		mysqlMetaData = mysqlMetaDataList[0]
	}

	foreignKeys := map[string]tableForeignKey{}
	{
		mysqlForeignKeys := []mysqlInfoTableForeignKeys{}
		if err := service.runSelect(ctx, statement{
			Query: `
				SELECT
					CONSTRAINT_NAME,
					COLUMN_NAME,
					REFERENCED_TABLE_NAME,
					REFERENCED_COLUMN_NAME
				FROM information_schema.key_column_usage
				WHERE
					referenced_table_schema = (SELECT DATABASE())
					AND table_name = :tableName;
			`,
			Parameters: map[string]any{
				":tableName": tableName,
			},
		}, &mysqlForeignKeys); err != nil {
			return Table{}, err
		}

		for _, mysqlForeignKey := range mysqlForeignKeys {
			foreignKeys[mysqlForeignKey.ColumnName] = tableForeignKey{
				TargetTable:  mysqlForeignKey.ReferencedTableName,
				TargetColumn: mysqlForeignKey.ReferencedColumnName,
			}
		}
	}

	columns := []TableColumn{}
	{
		mysqlColumns := []mysqlInfoTableColumn{}
		if err := service.runSelect(ctx, statement{
			Query: `
				SELECT
					COLUMN_NAME,
					COLUMN_TYPE,
					COLUMN_KEY,
					IS_NULLABLE,
					COLUMN_DEFAULT,
					EXTRA,
					COLUMN_COMMENT
				FROM
					information_schema.columns
				WHERE
					table_schema = :database
					AND table_name = :table
			`,
			Parameters: map[string]any{
				":database": driver.config.Name,
				":table":    tableName,
			},
		}, &mysqlColumns); err != nil {
			return Table{}, err
		}

		for _, mysqlColumn := range mysqlColumns {
			column := TableColumn{
				Name:       mysqlColumn.Name,
				PrimaryKey: mysqlColumn.Key == "PRI",
				Type: func() string {
					t := mysqlColumn.Type
					allowedToHaveLength := []string{
						"varchar",
					}

					for _, s := range allowedToHaveLength {
						if strings.HasPrefix(t, s) {
							return t
						}
					}

					t = regexp.MustCompile(`(?m)\(\d+\)`).ReplaceAllString(t, "")

					return t
				}(),
				Nullable:      mysqlColumn.Nullable == "YES",
				Comment:       mysqlColumn.Comment,
				AutoIncrement: strings.Contains(mysqlColumn.Extra, "auto_increment"),
				ForeignKey:    foreignKeys[mysqlColumn.Name],
				Default: func() *string {
					if mysqlColumn.Default == nil && mysqlColumn.Nullable == "YES" {
						x := "NULL"
						return &x

					}

					if mysqlColumn.Default != nil && *mysqlColumn.Default == "current_timestamp()" {
						x := "CURRENT_TIMESTAMP"
						return &x
					}

					return mysqlColumn.Default
				}(),
			}

			columns = append(columns, column)
		}
	}

	indexes := []TableIndex{}
	{
		mysqlIndexes := []mysqlInfoTableIndex{}
		if err := service.runSelect(ctx, statement{
			Query: `
				SELECT
					INDEX_NAME,
					COLUMN_NAME,
					NON_UNIQUE
				FROM
					information_schema.statistics
				WHERE
					table_schema = :database
					AND table_name = :table
					AND INDEX_NAME != "PRIMARY"
			`,
			Parameters: map[string]any{
				":database": driver.config.Name,
				":table":    tableName,
			},
		}, &mysqlIndexes); err != nil {
			return Table{}, err
		}

		lookupForDuplicates := map[string]int{}

		for _, mysqlIndex := range mysqlIndexes {
			existingIndexIndex, alreadySeenThisKey := lookupForDuplicates[mysqlIndex.Name]
			if alreadySeenThisKey {
				indexes[existingIndexIndex].Columns = append(indexes[existingIndexIndex].Columns, mysqlIndex.Column)
				continue
			}
			index := TableIndex{
				Name:    mysqlIndex.Name,
				Columns: []string{mysqlIndex.Column},
				Unique:  mysqlIndex.NonUnique == "0",
			}

			indexes = append(indexes, index)
			lookupForDuplicates[mysqlIndex.Name] = len(indexes) - 1
		}
	}

	return Table{
		Name:    tableName,
		Comment: mysqlMetaData.Comment,
		Indexes: indexes,
		columns: columns,
	}, nil
}

func (driver *driverMySQL) convertTypeBool() string {
	return "tinyint"
}

func (driver *driverMySQL) convertTypeInt() string {
	return "int"
}

func (driver *driverMySQL) convertTypeInt8() string {
	return "tinyint"
}

func (driver *driverMySQL) convertTypeInt16() string {
	return "smallint"
}

func (driver *driverMySQL) convertTypeInt32() string {
	return "int"
}

func (driver *driverMySQL) convertTypeInt64() string {
	return "bigint"
}

func (driver *driverMySQL) convertTypeUint() string {
	return "int unsigned"
}

func (driver *driverMySQL) convertTypeUint8() string {
	return "tinyint unsigned"
}

func (driver *driverMySQL) convertTypeUint16() string {
	return "smallint unsigned"
}

func (driver *driverMySQL) convertTypeUint32() string {
	return "int unsigned"
}

func (driver *driverMySQL) convertTypeUint64() string {
	return "bigint unsigned"
}

func (driver *driverMySQL) convertTypeFloat32() string {
	return "float"
}

func (driver *driverMySQL) convertTypeFloat64() string {
	return "double"
}

func (driver *driverMySQL) convertTypeString() string {
	return "varchar(255)"
}

func (driver *driverMySQL) convertTypeDateTime() string {
	return "datetime"
}

func (driver *driverMySQL) convertTypeJSON() string {
	return "longtext"
}

func (driver *driverMySQL) generateDelete(e Entity) (statement, error) {
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

func (driver *driverMySQL) generateInsert(e Entity) (statement, error) {
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

func (driver *driverMySQL) generateSelect(query Query) (statement, error) {
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

func (driver *driverMySQL) generateSimpleOperatorOfEquality(o simpleOperatorOfEquality) (statement, error) {
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

func (driver *driverMySQL) generateSimpleOperatorOfLogic(o simpleOperatorOfLogic) (statement, error) {
	return generateSimpleOperatorOfLogic(driver, o)
}

func (driver *driverMySQL) generateUpdate(e Entity) (statement, error) {
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

func (driver *driverMySQL) usesLastInsertId() bool {
	return true
}

func (driver *driverMySQL) usesNumberedParameters() bool {
	return false
}

type mysqlInfoTableMetaData struct {
	Name             string  `db:"Name"`
	Engine           string  `db:"Engine"`
	Version          string  `db:"Version"`
	Row_format       string  `db:"Row_format"`
	Rows             string  `db:"Rows"`
	Avg_row_length   string  `db:"Avg_row_length"`
	Data_length      string  `db:"Data_length"`
	Max_data_length  string  `db:"Max_data_length"`
	Index_length     string  `db:"Index_length"`
	Data_free        string  `db:"Data_free"`
	Auto_increment   *string `db:"Auto_increment"`
	Create_time      string  `db:"Create_time"`
	Update_time      *string `db:"Update_time"`
	Check_time       *string `db:"Check_time"`
	Collation        string  `db:"Collation"`
	Checksum         *string `db:"Checksum"`
	Create_options   string  `db:"Create_options"`
	Comment          string  `db:"Comment"`
	Max_index_length string  `db:"Max_index_length"`
	Temporary        string  `db:"Temporary"`
}

type mysqlInfoTableColumn struct {
	Name     string  `db:"COLUMN_NAME"`
	Key      string  `db:"COLUMN_KEY"`
	Type     string  `db:"COLUMN_TYPE"`
	Nullable string  `db:"IS_NULLABLE"`
	Default  *string `db:"COLUMN_DEFAULT"`
	Extra    string  `db:"EXTRA"`
	Comment  string  `db:"COLUMN_COMMENT"`
}

type mysqlInfoTableIndex struct {
	Name      string `db:"INDEX_NAME"`
	Column    string `db:"COLUMN_NAME"`
	NonUnique string `db:"NON_UNIQUE"`
}

type mysqlInfoTableForeignKeys struct {
	ConstraintName       string `db:"CONSTRAINT_NAME"`
	ColumnName           string `db:"COLUMN_NAME"`
	ReferencedTableName  string `db:"REFERENCED_TABLE_NAME"`
	ReferencedColumnName string `db:"REFERENCED_COLUMN_NAME"`
}

func (driver *driverMySQL) renderColumn(column TableColumn) (string, error) {
	defaultStuff := ""
	if column.Default != nil && *column.Default != "" {
		defaultStuff = " DEFAULT " + *column.Default
	}

	nullable := " NOT NULL"
	if column.Nullable {
		nullable = ""
	}

	extras := ""
	if column.PrimaryKey {
		extras = " PRIMARY KEY"
		if column.AutoIncrement {
			extras += " AUTO_INCREMENT"
		}
	}

	return fmt.Sprintf("`%s` %s%s%s%s COMMENT '%s'", column.Name, column.Type, defaultStuff, nullable, extras, column.Comment), nil
}

func (driver *driverMySQL) renderIndex(index TableIndex) (string, error) {
	columns := []string{}
	for _, column := range index.Columns {
		columns = append(columns, fmt.Sprintf("`%s`", column))
	}

	unique := ""
	if index.Unique {
		unique = "UNIQUE "
	}

	return fmt.Sprintf("%sKEY `%s` (%s)", unique, index.Name, strings.Join(columns, ", ")), nil
}

func (driver *driverMySQL) renderForeignKey(table Table, column TableColumn) (string, error) {
	return fmt.Sprintf(
		"CONSTRAINT `%s` FOREIGN KEY (`%s`) REFERENCES `%s` (`%s`) ON DELETE CASCADE",
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
	), nil
}
