package database

import (
	"context"
	"errors"
	"reflect"
	"slices"
)

func (service *Service) AutoMigrate(ctx context.Context, entities []Entity) (changesExecuted int, err error) {
	statements := []statement{}
	for _, entity := range entities {
		result := &migrationResult{}
		targetTable := entity.TableStructure()
		if err := targetTable.hydrateColumns(service.driver, entity); err != nil {
			return 0, err
		}
		targetTable = service.driver.autoMigrateAdjustTableDefinition(targetTable)

		sourceTable, err := service.driver.autoMigrateTableGet(ctx, service, targetTable.Name)
		if err != nil {
			if !errors.Is(err, ErrTableNotFound) {
				return 0, err
			} else {
				statements, err := service.driver.autoMigrateTableCreate(targetTable)
				if err != nil {
					return 0, err
				}

				result.TablesToAdd = append(result.TablesToAdd, statements...)

				sourceTable = targetTable
			}
		}

		tableDifferences, err := diff(sourceTable, targetTable)
		if err != nil {
			return 0, err
		}

		if err := result.DoTheThing(service.driver, tableDifferences); err != nil {
			if errors.Is(err, errNeedsAutoMigrateOverride) {
				statements = append(result.OverrideStatements, service.driver.autoMigrateOverride(sourceTable, targetTable)...)
			} else {
				return 0, err
			}
		} else {
			statements = append(statements, result.GetAllStatements()...)
		}
	}

	for i, statement := range statements {
		if _, err := service.runExecute(ctx, statement); err != nil {
			return i + 1, err
		}
	}

	return len(statements), nil
}

func diff(source Table, target Table) (tableDifferences, error) {
	sourceLookups := source.lookups()
	targetLookups := target.lookups()

	diff := tableDifferences{
		Table: target,
	}

	{ // Columns
		for name, target := range targetLookups.columns {
			source, found := sourceLookups.columns[name]
			// Add ones that need to be created
			if !found {
				diff.ColumnsToAdd = append(diff.ColumnsToAdd, target)
				continue
			}

			// Alter ones that are not correct in the source
			if !reflect.DeepEqual(source, target) {
				diff.ColumnsToAlter = append(diff.ColumnsToAlter, target)
				continue
			}
		}

		// Remove ones that should not exist
		for name, source := range sourceLookups.columns {
			if _, found := targetLookups.columns[name]; !found {
				diff.ColumnsToDrop = append(diff.ColumnsToDrop, source)
				continue
			}
		}
	}

	{ // Indexes
		for name, target := range targetLookups.indexes {
			source, found := sourceLookups.indexes[name]
			// Add ones that need to be created
			if !found {
				diff.IndexesToAdd = append(diff.IndexesToAdd, target)
				continue
			}

			// Alter ones that are not correct in the source
			slices.Sort(source.Columns)
			slices.Sort(target.Columns)
			if !reflect.DeepEqual(source, target) {
				diff.IndexesToAlter = append(diff.IndexesToAlter, target)
				continue
			}
		}

		// Remove ones that should not exist
		for name, source := range sourceLookups.indexes {
			if _, found := targetLookups.indexes[name]; !found {
				diff.IndexesToDrop = append(diff.IndexesToDrop, source)
				continue
			}
		}
	}

	return diff, nil
}

type tableDifferences struct {
	Table          Table
	ColumnsToAdd   []TableColumn
	ColumnsToAlter []TableColumn
	ColumnsToDrop  []TableColumn
	IndexesToAdd   []TableIndex
	IndexesToAlter []TableIndex
	IndexesToDrop  []TableIndex
}

func (diff tableDifferences) HasChanges() bool {
	if len(diff.ColumnsToAdd) > 0 {
		return true
	}
	if len(diff.ColumnsToAlter) > 0 {
		return true
	}
	if len(diff.ColumnsToDrop) > 0 {
		return true
	}
	if len(diff.IndexesToAdd) > 0 {
		return true
	}
	if len(diff.IndexesToAlter) > 0 {
		return true
	}
	if len(diff.IndexesToDrop) > 0 {
		return true
	}

	return false
}

type migrationResult struct {
	ColumnsToAdd       []statement
	ColumnsToAlter     []statement
	ColumnsToDrop      []statement
	IndexesToAdd       []statement
	IndexesToAlter     []statement
	IndexesToDrop      []statement
	TablesToAdd        []statement
	TablesToDrop       []statement
	OverrideStatements []statement
}

func (result *migrationResult) DoTheThing(driver Driver, diff tableDifferences) error {
	for _, x := range diff.ColumnsToAdd {
		statements, err := driver.autoMigrateColumnCreate(diff.Table, x)
		if err != nil {
			return err
		}

		result.ColumnsToAdd = append(result.ColumnsToAdd, statements...)
	}

	for _, x := range diff.ColumnsToAlter {
		statements, err := driver.autoMigrateColumnAlter(diff.Table, x)
		if err != nil {
			return err
		}

		result.ColumnsToAlter = append(result.ColumnsToAlter, statements...)
	}

	for _, x := range diff.ColumnsToDrop {
		statements, err := driver.autoMigrateColumnDrop(diff.Table, x)
		if err != nil {
			return err
		}

		result.ColumnsToDrop = append(result.ColumnsToDrop, statements...)
	}

	for _, x := range diff.IndexesToAdd {
		statements, err := driver.autoMigrateIndexCreate(diff.Table, x)
		if err != nil {
			return err
		}

		result.IndexesToAdd = append(result.IndexesToAdd, statements...)
	}

	for _, x := range diff.IndexesToAlter {
		statements, err := driver.autoMigrateIndexAlter(diff.Table, x)
		if err != nil {
			return err
		}

		result.IndexesToAlter = append(result.IndexesToAlter, statements...)
	}

	for _, x := range diff.IndexesToDrop {
		statements, err := driver.autoMigrateIndexDrop(diff.Table, x)
		if err != nil {
			return err
		}

		result.IndexesToDrop = append(result.IndexesToDrop, statements...)
	}

	return nil
}

func (results *migrationResult) GetAllStatements() []statement {

	return slices.Concat(
		// The order of these matter
		results.TablesToAdd,
		results.ColumnsToAdd,
		results.IndexesToAdd,
		results.ColumnsToAlter,
		results.IndexesToAlter,
		results.IndexesToDrop,
		results.ColumnsToDrop,
		results.TablesToDrop,
		results.OverrideStatements,
	)
}
