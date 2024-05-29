package models

import (
	"fmt"

	"github.com/go-mail/mail/v2"
)

const (
	DefaultSender = "www.sopin-l@mail.ru"
)

type Email struct {
	From      string
	To        string
	Subject   string
	Plaintext string
	HTML      string
}

type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
}

type EmailService struct {
	// DefaultSender is used as the default sender when one isn't provided for an
	// email. This is also used in functions where the email is a predetermined,
	// like the forgotten password email.
	DefaultSender string

	// unexported fields
	dialer *mail.Dialer
}

func NewEmailService(config SMTPConfig) *EmailService {
	es := EmailService{
		// TODO: Setup the fields, specifically the dialer
		dialer: mail.NewDialer(config.Host, config.Port, config.Username, config.Password),
	}
	return &es
}

func (es *EmailService) Send(email Email) error {
	msg := mail.NewMessage()
	msg.SetHeader("To", email.To)
	// TODO: Set the From field to a default if it is not set
	es.setFrom(msg, email)
	msg.SetHeader("Subject", email.Subject)
	switch {
	case email.Plaintext != "" && email.HTML != "":
		msg.SetBody("text/plain", email.Plaintext)
		msg.AddAlternative("text/html", email.HTML)
	case email.Plaintext != "":
		msg.SetBody("text/plain", email.Plaintext)
	case email.HTML != "":
		msg.SetBody("text/html", email.HTML)
	}
	err := es.dialer.DialAndSend(msg)
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	return nil
}

func (es *EmailService) ForgotPassword(to, resetURL string) error {
	email := Email{
		From:      "www.sopin-l@mail.ru",
		Subject:   "Reset your password",
		To:        to,
		Plaintext: "To reset your password, please visit the following link:" + resetURL,
		HTML: `<p> To reset your password, please visit the following link: <a href="` + resetURL + `">` +
			resetURL + `</a></p>`,
	}
	if err := es.Send(email); err != nil {
		return fmt.Errorf("forgot password email: %w", err)
	}
	return nil

}

func (es *EmailService) setFrom(msg *mail.Message, email Email) {
	var from string
	switch {
	case email.From != "":
		from = email.From
	case es.DefaultSender != "":
		from = es.DefaultSender
	default:
		from = DefaultSender
	}
	msg.SetHeader("From", from)
}
