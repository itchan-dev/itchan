package service

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/itchan-dev/itchan/backend/internal/utils"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/itchan-dev/itchan/shared/errors"
	"golang.org/x/crypto/bcrypt"
)

type AuthService interface {
	Register(creds domain.Credentials) error
	CheckConfirmationCode(email domain.Email, confirmationCode string) error
	Login(creds domain.Credentials) (string, error)
}

type Auth struct {
	storage AuthStorage
	email   Email
	jwt     Jwt
}

type AuthStorage interface {
	SaveUser(user domain.User) (domain.UserId, error)
	User(email domain.Email) (domain.User, error)
	UpdatePassword(creds domain.Credentials) error
	DeleteUser(email domain.Email) error
	SaveConfirmationData(data domain.ConfirmationData) error
	ConfirmationData(email domain.Email) (domain.ConfirmationData, error)
	DeleteConfirmationData(email domain.Email) error
}

type Email interface {
	Send(recipientEmail, subject, body string) error
	IsCorrect(email domain.Email) error
}

type Jwt interface {
	NewToken(user domain.User) (string, error)
}

func NewAuth(storage AuthStorage, email Email, jwt Jwt) *Auth {
	return &Auth{storage, email, jwt}
}

// Generate and send confirmation code to destinated email
// Save email, passHash, confirmation code to database
func (a *Auth) Register(creds domain.Credentials) error {
	email := strings.ToLower(creds.Email)

	var err error

	err = a.email.IsCorrect(email)
	if err != nil {
		return err
	}

	cData, err := a.storage.ConfirmationData(email)
	if err != nil && !errors.IsNotFound(err) { // if there is error, and error is not "not found"
		return err
	}
	if err == nil { // data presented, check expiration
		if cData.Expires.Before(time.Now()) { // if data expired - delete
			if err := a.storage.DeleteConfirmationData(email); err != nil {
				return err
			}
		} else {
			diff := cData.Expires.Sub(time.Now())
			return &errors.ErrorWithStatusCode{Message: fmt.Sprintf("Previous confirmation code is still valid. Retry after %.0fs", diff.Seconds()), StatusCode: http.StatusTooEarly}
		}
	}

	confirmationCode := utils.GenerateConfirmationCode(6)
	passHash, err := bcrypt.GenerateFromPassword([]byte(creds.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Print(err.Error())
		return err
	}
	confirmationCodeHash, err := bcrypt.GenerateFromPassword([]byte(confirmationCode), bcrypt.DefaultCost)
	if err != nil {
		log.Print(err.Error())
		return err
	}
	err = a.storage.SaveConfirmationData(domain.ConfirmationData{Email: email, NewPassHash: string(passHash), ConfirmationCodeHash: string(confirmationCodeHash), Expires: time.Now().UTC().Add(5 * time.Minute)})
	if err != nil {
		return err
	}

	emailBody := fmt.Sprintf(`
		Hello,

		Your confirmation code below

		%s

		If you did not request this, please ignore this email.
	`, confirmationCode)

	err = a.email.Send(email, "Please confirm your email address", emailBody)
	if err != nil {
		return err
	}
	return nil
}

// Confirm code sended via Register func and update user password
func (a *Auth) CheckConfirmationCode(email domain.Email, confirmationCode string) error {
	email = strings.ToLower(email)

	if err := a.email.IsCorrect(email); err != nil {
		return err
	}

	data, err := a.storage.ConfirmationData(email)
	if err != nil {
		return err
	}
	if data.Expires.Before(time.Now()) {
		return &errors.ErrorWithStatusCode{Message: "Confirmation time expired", StatusCode: http.StatusBadRequest}
	}
	if err := bcrypt.CompareHashAndPassword([]byte(data.ConfirmationCodeHash), []byte(confirmationCode)); err != nil {
		log.Print(err.Error())
		return &errors.ErrorWithStatusCode{Message: "Wrong confirmation code", StatusCode: http.StatusBadRequest}
	}
	// if not exists - create
	_, err = a.storage.User(email)
	if err != nil {
		e, ok := err.(*errors.ErrorWithStatusCode)
		if ok && e.StatusCode == http.StatusNotFound {
			if _, err := a.storage.SaveUser(domain.User{Email: email, PassHash: data.NewPassHash}); err != nil {
				return err
			}
		} else {
			return err
		}
	} else {
		if err := a.storage.UpdatePassword(domain.Credentials{Email: email, Password: data.NewPassHash}); err != nil {
			return err
		}
	}
	if err := a.storage.DeleteConfirmationData(email); err != nil { // cleanup
		return err
	}
	return nil
}

// Login checks if user with given credentials exists in the system and returns access token.
// If user exists, but password is incorrect, returns error.
// If user doesn't exist, returns error.
func (a *Auth) Login(creds domain.Credentials) (string, error) {
	email := strings.ToLower(creds.Email)
	password := creds.Password

	err := a.email.IsCorrect(email)
	if err != nil {
		return "", err
	}

	user, err := a.storage.User(email)
	if err != nil {
		// to not leak existing users
		e, ok := err.(*errors.ErrorWithStatusCode)
		if ok && e.StatusCode == http.StatusNotFound {
			return "", &errors.ErrorWithStatusCode{
				Message:    "Invalid credentials",
				StatusCode: http.StatusUnauthorized,
			}
		}
		return "", err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PassHash), []byte(password))
	if err != nil {
		log.Print(err.Error())
		return "", &errors.ErrorWithStatusCode{Message: "Invalid credentials", StatusCode: http.StatusUnauthorized}
	}

	token, err := a.jwt.NewToken(user)
	if err != nil {
		log.Print(err.Error())
		return "", err
	}

	return token, nil
}
