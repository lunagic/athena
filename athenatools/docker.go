package athenatools

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"testing"

	"github.com/ory/dockertest"
)

type DockerServiceConfig[T any] struct {
	DockerImage    string
	DockerImageTag string
	InternalPort   int
	Environment    map[string]string
	Builder        func(host string, port int) (T, error)
}

func (d DockerServiceConfig[T]) Env() []string {
	env := []string{}
	for k, v := range d.Environment {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	return env
}

func GetDockerService[T any](
	t *testing.T,
	context DockerServiceConfig[T],
) T {
	t.Helper()
	if testing.Short() {
		t.Skip("Skipping long-running test in short mode.")
	}

	// uses a sensible default on windows (tcp/http) and linux/osx (socket)
	pool, err := dockertest.NewPool("")
	if err != nil {
		t.Fatalf("Could not construct pool: %s", err)
	}

	// uses pool to try to connect to Docker
	err = pool.Client.Ping()
	if err != nil {
		t.Fatalf("Could not connect to Docker: %s", err)
	}

	// pulls an image, creates a container based on it and runs it
	resource, err := pool.Run(
		context.DockerImage,
		context.DockerImageTag,
		context.Env(),
	)

	if err != nil {
		t.Fatalf("Could not start resource: %s", err)
	}

	t.Cleanup(func() {
		if err := pool.Purge(resource); err != nil {
			t.Fatalf("Could not purge resource: %s", err)
		}
	})

	dockerURL := os.Getenv("DOCKER_HOST")
	if dockerURL == "" {
		dockerURL = "tcp://" + resource.GetHostPort(fmt.Sprintf("%d/tcp", context.InternalPort))
	}
	u, err := url.Parse(dockerURL)
	if err != nil {
		t.Fatalf("Error parsing docker URL: %s", err)
	}

	port := func() int {
		i, _ := strconv.Atoi(u.Port())
		return i
	}()

	var db T

	if err := pool.Retry(func() error {
		var err error

		db, err = context.Builder(u.Hostname(), port)
		if err != nil {
			return err
		}

		return nil
	}); err != nil {
		t.Fatalf("Could not connect to database: %s", err)
	}

	return db
}
