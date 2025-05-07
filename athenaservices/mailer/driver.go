package mailer

import (
	"fmt"
	"slices"
	"strings"

	"github.com/lunagic/athena/athenatools"
)

type EnvelopeTarget struct {
	Name  string
	Email string
}

func (target EnvelopeTarget) String() string {
	if target.Name == "" {
		return target.Email
	}

	return fmt.Sprintf("%s <%s>", target.Name, target.Email)
}

type Envelope struct {
	To      []EnvelopeTarget
	CC      []EnvelopeTarget
	BCC     []EnvelopeTarget
	From    EnvelopeTarget
	Subject string
	Body    string
}

func (envelope Envelope) Destinations() ([]string, error) {
	destinations := []string{}
	for _, to := range slices.Concat(envelope.To, envelope.CC, envelope.BCC) {
		destinations = append(destinations, to.Email)
	}
	return destinations, nil
}

func (envelope Envelope) Message() ([]byte, error) {
	header := map[string]string{}

	header["From"] = envelope.From.String()
	header["Subject"] = envelope.Subject
	header["To"] = strings.Join(
		athenatools.Map(envelope.To, func(x EnvelopeTarget) string {
			return x.String()
		}),
		", ",
	)
	header["CC"] = strings.Join(
		athenatools.Map(envelope.CC, func(x EnvelopeTarget) string {
			return x.String()
		}),
		", ",
	)

	message := ""
	for key, value := range header {
		message += fmt.Sprintf("%s: %s\r\n", key, value)
	}
	return []byte(message + "\r\n" + envelope.Body), nil
}

type Driver interface {
	Send(envelope Envelope) error
}
