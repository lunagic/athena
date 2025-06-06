package queue_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/lunagic/athena/athenaservices/queue"
	"github.com/lunagic/athena/athenatest"
)

func TestDriverRabbitMQ(t *testing.T) {
	user := uuid.NewString()
	pass := uuid.NewString()

	testSuite(t, athenatest.GetDockerService(t,
		athenatest.DockerServiceConfig[queue.Driver]{
			DockerImage:    "rabbitmq",
			DockerImageTag: "3",
			InternalPort:   5672,
			Environment: map[string]string{
				"RABBITMQ_DEFAULT_USER": user,
				"RABBITMQ_DEFAULT_PASS": pass,
			},
			Builder: func(host string, port int) (queue.Driver, error) {
				return queue.NewDriverRabbitMQ(queue.DriverRabbitMQConfig{
					Host: host,
					Port: port,
					User: user,
					Pass: pass,
				})
			},
		},
	))
}
