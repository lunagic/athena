package athena

import "github.com/lunagic/athena/athenaservices/database"

func WithDatabaseAutoMigration(databaseService *database.Service, entities []database.Entity) ConfigurationFunc {
	return func(app *App) error {
		app.database = databaseService
		app.databaseAutoMigrationEntities = entities

		return nil
	}
}
