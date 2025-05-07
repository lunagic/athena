package mailer_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lunagic/athena/athenaservices/mailer"
	"github.com/lunagic/athena/athenatools"
)

func TestSMTP_Gmail(t *testing.T) {
	t.Parallel()

	config := mailer.DriverSMTPConfig{
		Host: "smtp.gmail.com",
		Port: 25,
		Name: os.Getenv("ATHENA_TEST_MAILER_GMAIL_NAME"),
		User: os.Getenv("ATHENA_TEST_MAILER_GMAIL_EMAIL"),
		Pass: os.Getenv("ATHENA_TEST_MAILER_GMAIL_PASSWORD"),
	}

	driver, err := mailer.NewDriverSMTP(config)
	if err != nil {
		t.Fatal(err)
	}

	if config.User == "" || config.Pass == "" {
		t.Skip()
	}

	testSuite(t, driver)
}

func TestSMTP_Mailpit(t *testing.T) {
	user := uuid.NewString()
	password := uuid.NewString()

	driver := athenatools.GetDockerService(t, athenatools.DockerServiceConfig[mailer.Driver]{
		DockerImage:    "axllent/mailpit",
		DockerImageTag: "latest",
		InternalPort:   1025,
		Environment: map[string]string{
			"MP_SMTP_AUTH":                fmt.Sprintf("%s:%s", user, password),
			"MP_SMTP_AUTH_ALLOW_INSECURE": "1",
		},
		Builder: func(host string, port int) (mailer.Driver, error) {
			time.Sleep(time.Second)

			config := mailer.DriverSMTPConfig{
				Host: host,
				Port: port,
				Name: "Mailer Test",
				User: user,
				Pass: password,
			}

			driver, err := mailer.NewDriverSMTP(config)
			if err != nil {
				return nil, err
			}

			if err := driver.Send(mailer.Envelope{}); err != nil {
				if err.Error() == "EOF" {
					return nil, err
				}
			}

			return driver, nil
		},
	})

	testSuite(t, driver)
}

func testSuite(t *testing.T, driver mailer.Driver) {
	if err := driver.Send(mailer.Envelope{
		To: []mailer.EnvelopeTarget{
			{
				Name:  "Aaron Ellington",
				Email: "aaron@ellington.io",
			},
		},
		Subject: "This is a test",
		Body:    "Hello there!",
	}); err != nil {
		t.Fatal(err)
	}
}
