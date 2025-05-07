package queue

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type DriverRabbitMQConfig struct {
	Host string
	Pass string
	Port int
	User string
}

func NewDriverRabbitMQ(config DriverRabbitMQConfig) (Driver, error) {
	connection, err := amqp.Dial(fmt.Sprintf("amqp://%s:%s@%s:%d", config.User, config.Pass, config.Host, config.Port))
	if err != nil {
		return nil, err
	}

	channel, err := connection.Channel()
	if err != nil {
		return nil, err
	}

	return &driverRabbitMQ{
		channel: channel,
	}, nil
}

type driverRabbitMQ struct {
	channel *amqp.Channel
}

func (driver *driverRabbitMQ) CreateQueue(ctx context.Context, queueName string) error {
	_, err := driver.channel.QueueDeclare(
		queueName, // name
		false,     // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)

	return err
}

func (driver *driverRabbitMQ) Publish(ctx context.Context, queueName string, payload []byte) error {
	return driver.channel.PublishWithContext(
		ctx,
		"",        // exchange
		queueName, // routing key
		true,      // mandatory
		false,     // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        payload,
		},
	)
}

func (driver *driverRabbitMQ) Consume(
	ctx context.Context,
	queueName string,
	handler func(ctx context.Context, payload []byte) error,
) error {

	msgs, err := driver.channel.Consume(
		queueName, // queue
		"",        // consumer
		true,      // auto-ack
		false,     // exclusive
		false,     // no-local
		false,     // no-wait
		nil,       // args
	)
	if err != nil {
		return err
	}

	for d := range msgs {
		if err := handler(ctx, d.Body); err != nil {
			return err
		}
	}

	return nil
}
