package cache_test

import (
	"errors"
	"log"
	"testing"

	"github.com/lunagic/athena/athenaservices/cache"
	"github.com/lunagic/athena/athenatools"
)

func TestDriverRedis(t *testing.T) {
	t.Parallel()
	testRedisLikeCacheDrivers(t, "redis", "latest")
}

func TestDriverValkey(t *testing.T) {
	t.Parallel()
	testRedisLikeCacheDrivers(t, "valkey/valkey", "latest")
}

func TestDriverKeyDB(t *testing.T) {
	t.Parallel()
	testRedisLikeCacheDrivers(t, "eqalpha/keydb", "latest")
}

func TestDriverGarnet(t *testing.T) {
	t.Parallel()
	testRedisLikeCacheDrivers(t, "ghcr.io/microsoft/garnet", "latest")
}

func TestDriverDragonfly(t *testing.T) {
	t.Parallel()
	testRedisLikeCacheDrivers(t, "docker.dragonflydb.io/dragonflydb/dragonfly", "latest")
}

func testRedisLikeCacheDrivers(t *testing.T, image string, tag string) {
	driver := athenatools.GetDockerService(
		t,
		athenatools.DockerServiceConfig[cache.Driver]{
			DockerImage:    image,
			DockerImageTag: tag,
			InternalPort:   6379,
			Environment:    map[string]string{},
			Builder: func(host string, port int) (cache.Driver, error) {
				driver, err := cache.NewDriverRedis(
					cache.DriverRedisConfig{
						Host: host,
						Port: port,
					},
				)
				if err != nil {
					return nil, err
				}

				if _, err := driver.Get(t.Context(), "example"); err != nil {
					if !errors.Is(err, cache.ErrNotFound) {
						log.Println(err)
						return nil, err
					}
				}

				return driver, nil
			},
		},
	)

	testCase(t, driver)
}
