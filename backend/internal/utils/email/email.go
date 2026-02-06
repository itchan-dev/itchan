package email

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"time"

	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/errors"
	"github.com/itchan-dev/itchan/shared/logger"
)

type Email struct {
	config *config.Email
	auth   smtp.Auth
}

func New(config *config.Email) *Email {
	auth := smtp.PlainAuth("", config.Username, config.Password, config.SMTPServer)
	return &Email{
		config: config,
		auth:   auth,
	}
}

func (e *Email) IsCorrect(email string) error {
	_, err := mail.ParseAddress(email)
	if err != nil {
		return &errors.ErrorWithStatusCode{Message: err.Error(), StatusCode: 400}
	}
	return nil
}

func (e *Email) Send(recipientEmail, subject, body string) error {
	msg := e.buildMessage(recipientEmail, subject, body)
	address := fmt.Sprintf("%s:%d", e.config.SMTPServer, e.config.SMTPPort)

	// Port 465 = implicit TLS, otherwise STARTTLS
	if e.config.SMTPPort == 465 {
		return e.sendImplicitTLS(address, recipientEmail, msg)
	}
	return e.sendSTARTTLS(address, recipientEmail, msg)
}

func (e *Email) timeout() time.Duration {
	timeout := time.Duration(e.config.Timeout) * time.Second
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return timeout
}

// sendImplicitTLS sends email over a connection that is TLS from the start (port 465).
func (e *Email) sendImplicitTLS(address, recipientEmail string, msg []byte) error {
	tlsConfig := &tls.Config{ServerName: e.config.SMTPServer}

	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: e.timeout()}, "tcp", address, tlsConfig)
	if err != nil {
		logger.Log.Error("failed to connect to SMTP server (implicit TLS)", "address", address, "error", err)
		return err
	}
	defer conn.Close()

	return e.sendOverConn(conn, recipientEmail, msg)
}

// sendSTARTTLS sends email by upgrading a plain connection to TLS (port 587).
func (e *Email) sendSTARTTLS(address, recipientEmail string, msg []byte) error {
	conn, err := net.DialTimeout("tcp", address, e.timeout())
	if err != nil {
		logger.Log.Error("failed to connect to SMTP server", "address", address, "error", err)
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, e.config.SMTPServer)
	if err != nil {
		logger.Log.Error("failed to create SMTP client", "error", err)
		return err
	}
	defer client.Close()

	tlsConfig := &tls.Config{ServerName: e.config.SMTPServer}
	if err = client.StartTLS(tlsConfig); err != nil {
		logger.Log.Error("failed to start TLS", "error", err)
		return err
	}

	return e.sendViaClient(client, recipientEmail, msg)
}

// sendOverConn creates an SMTP client from an existing connection and sends the message.
func (e *Email) sendOverConn(conn net.Conn, recipientEmail string, msg []byte) error {
	client, err := smtp.NewClient(conn, e.config.SMTPServer)
	if err != nil {
		logger.Log.Error("failed to create SMTP client", "error", err)
		return err
	}
	defer client.Close()

	return e.sendViaClient(client, recipientEmail, msg)
}

// sendViaClient performs auth, sets sender/recipient, and sends the message body.
func (e *Email) sendViaClient(client *smtp.Client, recipientEmail string, msg []byte) error {
	if err := client.Auth(e.auth); err != nil {
		logger.Log.Error("SMTP authentication failed", "error", err)
		return err
	}

	if err := client.Mail(e.config.Username); err != nil {
		logger.Log.Error("failed to set sender", "error", err)
		return err
	}

	if err := client.Rcpt(recipientEmail); err != nil {
		logger.Log.Error("failed to set recipient", "recipient", recipientEmail, "error", err)
		return err
	}

	w, err := client.Data()
	if err != nil {
		logger.Log.Error("failed to get data writer", "error", err)
		return err
	}

	if _, err = w.Write(msg); err != nil {
		logger.Log.Error("failed to write message", "error", err)
		return err
	}

	if err = w.Close(); err != nil {
		logger.Log.Error("failed to close data writer", "error", err)
		return err
	}

	return client.Quit()
}

func (e *Email) buildMessage(recipient, subject, body string) []byte {
	return []byte(fmt.Sprintf(
		"To: %s\r\nFrom: %s <%s>\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=\"utf-8\"\r\n\r\n%s",
		recipient,
		e.config.SenderName,
		e.config.Username,
		subject,
		body,
	))
}
