package mailer

import (
	"fmt"
	"net/smtp"
)

func NewDriverSMTP(config DriverSMTPConfig) (Driver, error) {
	return &driverSMTP{
		config: config,
	}, nil
}

type DriverSMTPConfig struct {
	Host string
	Port int
	User string
	Pass string
	Name string
}

type driverSMTP struct {
	config DriverSMTPConfig
}

func (driver driverSMTP) Send(envelope Envelope) error {
	envelope.From.Email = driver.config.User
	if envelope.From.Name == "" {
		envelope.From.Name = driver.config.Name
	}

	auth := smtp.PlainAuth(
		"identity",
		driver.config.User,
		driver.config.Pass,
		driver.config.Host,
	)

	destinations, err := envelope.Destinations()
	if err != nil {
		return err
	}

	message, err := envelope.Message()
	if err != nil {
		return err
	}

	if err := smtp.SendMail(
		fmt.Sprintf("%s:%d", driver.config.Host, driver.config.Port),
		auth,
		driver.config.User,
		destinations,
		message,
	); err != nil {
		return err
	}

	return nil
}
