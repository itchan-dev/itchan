package email

import (
	"fmt"
	"log"
	"net/mail"
	"net/smtp"

	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/errors"
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
	err := smtp.SendMail(address, e.auth, e.config.SenderName, []string{recipientEmail}, msg)
	if err != nil {
		log.Fatal("Error sending email:", err)
	}

	return nil
}

// func (e *Email) Close() error {
// 	e.mu.Lock()
// 	defer e.mu.Unlock()

// 	var errs []error
// 	if e.client != nil {
// 		if err := e.client.Quit(); err != nil {
// 			errs = append(errs, fmt.Errorf("SMTP quit error: %w", err))
// 		}
// 		e.client = nil
// 	}
// 	if e.conn != nil {
// 		if err := e.conn.Close(); err != nil {
// 			errs = append(errs, fmt.Errorf("connection close error: %w", err))
// 		}
// 		e.conn = nil
// 	}

// 	if len(errs) > 0 {
// 		return fmt.Errorf("error closing connection: %v", errs)
// 	}
// 	return nil
// }

// func (e *Email) resetConnection() {
// 	if e.client != nil {
// 		e.client.Close()
// 		e.client = nil
// 	}
// 	if e.conn != nil {
// 		e.conn.Close()
// 		e.conn = nil
// 	}
// }

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

// func (e *Email) connect() (net.Conn, error) {
// 	address := fmt.Sprintf("%s:%d", e.config.SMTPServer, e.config.SMTPPort)
// 	dialer := net.Dialer{Timeout: time.Duration(e.config.Timeout) * time.Second}

// 	if e.config.UseTLS {
// 		tlsConfig := &tls.Config{
// 			ServerName:         e.config.SMTPServer,
// 			InsecureSkipVerify: e.config.InsecureSkipVerify,
// 		}
// 		return tls.DialWithDialer(&dialer, "tcp", address, tlsConfig)
// 	}

// 	return dialer.Dial("tcp", address)
// }
