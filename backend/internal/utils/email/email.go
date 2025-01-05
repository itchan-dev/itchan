package email

// placeholder code

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"

	"github.com/itchan-dev/itchan/backend/internal/errors"
	"github.com/itchan-dev/itchan/shared/config"
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

func (s *Email) Send(recipientEmail, subject, body string) error {
	msg := []byte(fmt.Sprintf("To: %s\r\n"+
		"From: %s <%s>\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/plain; charset=\"utf-8\"\r\n\r\n"+
		"%s",
		recipientEmail, s.config.SenderName, s.config.Username, subject, body))

	// Establish connection (using a helper function)
	conn, err := s.connect()
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer conn.Close()

	// Create SMTP client
	client, err := smtp.NewClient(conn, s.config.SMTPServer)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Close()

	// Perform authentication
	if err := client.Auth(s.auth); err != nil {
		return fmt.Errorf("failed to authenticate: %w", err)
	}

	// Send the email
	if err := client.Mail(s.config.Username); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}
	if err := client.Rcpt(recipientEmail); err != nil {
		return fmt.Errorf("failed to set recipient: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to open data writer: %w", err)
	}
	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("failed to write message body: %w", err)
	}
	err = w.Close()
	if err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	// Quit the SMTP session
	if err := client.Quit(); err != nil {
		return fmt.Errorf("failed to quit SMTP session: %w", err)
	}

	return nil
}

func (s *Email) connect() (net.Conn, error) {
	address := fmt.Sprintf("%s:%d", s.config.SMTPServer, s.config.SMTPPort)

	dialer := net.Dialer{
		Timeout: s.config.Timeout,
	}

	if s.config.UseTLS {
		tlsConfig := &tls.Config{
			ServerName:         s.config.SMTPServer,
			InsecureSkipVerify: s.config.InsecureSkipVerify, // Set to true only for testing!
		}
		conn, err := tls.DialWithDialer(&dialer, "tcp", address, tlsConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to establish TLS connection: %w", err)
		}
		return conn, nil
	}

	conn, err := dialer.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to establish connection: %w", err)
	}
	return conn, nil
}
