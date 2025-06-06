package database_test

// ================================================================
// ================================================================
// ============================= DONE =============================
// ================================================================
// ================================================================

import (
	"testing"

	"github.com/google/uuid"
	"github.com/lunagic/athena/athenaservices/database"
	"github.com/lunagic/athena/athenatest"
)

func Test_DriverMySQL_8(t *testing.T) {
	t.Parallel()
	testSuite(t, setupMySQL(t, "mysql", "8"))
}

func Test_DriverMySQL_MariaDB_11_4(t *testing.T) {
	t.Parallel()
	testSuite(t, setupMySQL(t, "mariadb", "11.4"))
}

func Test_DriverMySQL_MariaDB_10_11(t *testing.T) {
	t.Parallel()
	testSuite(t, setupMySQL(t, "mariadb", "10.11"))
}

func Test_DriverMySQL_MariaDB_10_6(t *testing.T) {
	t.Parallel()
	testSuite(t, setupMySQL(t, "mariadb", "10.6"))
}

func setupMySQL(
	t *testing.T,
	image string,
	tag string,
) database.Driver {
	name := uuid.NewString()
	pass := uuid.NewString()
	user := uuid.NewString()[0:32] // MySQL can't have usernames longer than 32 characters

	return athenatest.GetDockerService(
		t,
		athenatest.DockerServiceConfig[database.Driver]{
			DockerImage:    image,
			DockerImageTag: tag,
			InternalPort:   3306,
			Environment: map[string]string{
				"MYSQL_ROOT_PASSWORD": uuid.NewString(),
				"MYSQL_PASSWORD":      pass,
				"MYSQL_DATABASE":      name,
				"MYSQL_USER":          user,
			},
			Builder: func(host string, port int) (database.Driver, error) {
				driver := database.NewDriverMySQL(database.DriverMySQLConfig{
					Host: host,
					Port: port,
					User: user,
					Pass: pass,
					Name: name,
				})

				db, err := driver.Open()
				if err != nil {
					return nil, err
				}

				if err := db.Ping(); err != nil {
					return nil, err
				}

				return driver, nil
			},
		},
	)
}
