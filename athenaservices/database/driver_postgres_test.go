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

func Test_DriverPostgres_17(t *testing.T) {
	t.Parallel()
	testSuite(t, setupPostgres(t, "postgres", "17"))
}

func Test_DriverPostgres_16(t *testing.T) {
	t.Parallel()
	testSuite(t, setupPostgres(t, "postgres", "16"))
}

func Test_DriverPostgres_15(t *testing.T) {
	t.Parallel()
	testSuite(t, setupPostgres(t, "postgres", "15"))
}

func setupPostgres(
	t *testing.T,
	image string,
	tag string,
) database.Driver {
	name := uuid.NewString()
	pass := uuid.NewString()
	user := uuid.NewString()

	return athenatest.GetDockerService(
		t,
		athenatest.DockerServiceConfig[database.Driver]{
			DockerImage:    image,
			DockerImageTag: tag,
			InternalPort:   5432,
			Environment: map[string]string{
				"POSTGRES_USER":     user,
				"POSTGRES_PASSWORD": pass,
				"POSTGRES_DB":       name,
			},
			Builder: func(host string, port int) (database.Driver, error) {
				driver := database.NewDriverPostgres(database.DriverPostgresConfig{
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
