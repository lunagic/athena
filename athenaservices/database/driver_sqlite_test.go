package database_test

import (
	"fmt"
	"log"
	"testing"

	"github.com/lunagic/athena/athenaservices/database"
)

func TestSQLite(t *testing.T) {
	t.Parallel()
	/*
		Need to move FK to the TableColumn
		Need to improve PrimaryKey handling
	*/
	dbPath := fmt.Sprintf("%s/database.sqlite", t.TempDir())
	log.Println(dbPath)
	testSuite(t, database.NewDriverSQLite(dbPath))
}
