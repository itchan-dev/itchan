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
	Register(email, password string) error
	CheckConfirmationCode(email, confirmationCode string) error
	Login(email, password string) (string, error)
}

type Auth struct {
	storage AuthStorage
	email   Email
	jwt     Jwt
}

type AuthStorage interface {
	SaveUser(user *domain.User) (int64, error)
	User(email string) (*domain.User, error)
	UpdatePassword(email, passHash string) error
	DeleteUser(email string) error
	SaveConfirmationData(newPassword *domain.ConfirmationData) error
	ConfirmationData(email string) (*domain.ConfirmationData, error)
	DeleteConfirmationData(email string) error
}

type Email interface {
	Send(recipientEmail, subject, body string) error
	IsCorrect(email string) error
}

type Jwt interface {
	NewToken(user *domain.User) (string, error)
}

func NewAuth(storage AuthStorage, email Email, jwt Jwt) *Auth {
	return &Auth{storage, email, jwt}
}

// Generate and send confirmation code to destinated email
// Save email, passHash, confirmation code to database
func (a *Auth) Register(email, password string) error {
	email = strings.ToLower(email)

	var err error

	err = a.email.IsCorrect(email)
	if err != nil {
		return err
	}

	cData, err := a.storage.ConfirmationData(email)
	if cData != nil { // data presented
		if cData.Expires.Before(time.Now()) {
			if err := a.storage.DeleteConfirmationData(email); err != nil {
				return err
			}
		} else {
			diff := cData.Expires.Sub(time.Now())
			return &errors.ErrorWithStatusCode{Message: fmt.Sprintf("Previous confirmation code is still valid. Retry after %.0fs", diff.Seconds()), StatusCode: http.StatusTooEarly}
		}
	}

	confirmationCode := utils.GenerateConfirmationCode(6)
	passHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Print(err.Error())
		return err
	}
	confirmationCodeHash, err := bcrypt.GenerateFromPassword([]byte(confirmationCode), bcrypt.DefaultCost)
	if err != nil {
		log.Print(err.Error())
		return err
	}
	err = a.storage.SaveConfirmationData(&domain.ConfirmationData{Email: email, NewPassHash: string(passHash), ConfirmationCodeHash: string(confirmationCodeHash), Expires: time.Now().UTC().Add(5 * time.Minute)})
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
func (a *Auth) CheckConfirmationCode(email, confirmationCode string) error {
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
	user, err := a.storage.User(email)
	if err != nil {
		e, ok := err.(*errors.ErrorWithStatusCode)
		if ok && e.StatusCode == http.StatusNotFound {
			if _, err := a.storage.SaveUser(&domain.User{Email: email, PassHash: data.NewPassHash}); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	if user != nil {
		if err := a.storage.UpdatePassword(email, data.NewPassHash); err != nil {
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
func (a *Auth) Login(email, password string) (string, error) {
	email = strings.ToLower(email)

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
