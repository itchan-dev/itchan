package service

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/itchan-dev/itchan/backend/internal/errors"
	"github.com/itchan-dev/itchan/backend/internal/utils"
	"github.com/itchan-dev/itchan/shared/domain"
	"golang.org/x/crypto/bcrypt"
)

type AuthService interface {
	Register(email, password string) (int64, error)
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
	DeleteUser(email string) error
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
// Saving email, passHash, confirmation code to database
func (a *Auth) Register(email, password string) (int64, error) {
	var err error

	err = a.email.IsCorrect(email)
	if err != nil {
		return 0, err
	}

	confirmationCode := utils.GenerateConfirmationCode(6)
	emailBody := fmt.Sprintf(`
		Hello,

		Your confirmation code below

		%s

		If you did not request this, please ignore this email.
	`, confirmationCode)

	err = a.email.Send(email, "Please confirm your email address", emailBody)
	if err != nil {
		return 0, err
	}

	passHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Print(err.Error())
		return 0, err
	}
	confirmationCodeHash, err := bcrypt.GenerateFromPassword([]byte(confirmationCode), bcrypt.DefaultCost)
	if err != nil {
		log.Print(err.Error())
		return 0, err
	}

	uid, err := a.storage.SaveUser(&domain.User{Email: email, PassHash: string(passHash), ConfirmationCodeHash: string(confirmationCodeHash), ConfirmationExpires: time.Now().Add(5 * time.Minute)})
	if err != nil {
		return 0, err
	}

	return uid, nil
}

func (a *Auth) CheckConfirmationCode(email, confirmationCode string) error {
	err := a.email.IsCorrect(email)
	if err != nil {
		return err
	}

	user, err := a.storage.User(email)
	if err != nil {
		return err
	}
	if user.ConfirmationExpires.Before(time.Now()) {
		return &errors.ErrorWithStatusCode{Message: "Confirmation time expired", StatusCode: http.StatusBadRequest}
	}
	err = bcrypt.CompareHashAndPassword([]byte(user.ConfirmationCodeHash), []byte(confirmationCode))
	if err != nil {
		log.Print(err.Error())
		return &errors.ErrorWithStatusCode{Message: "Wrong confirmation code", StatusCode: http.StatusBadRequest}
	}
	return nil
}

// Login checks if user with given credentials exists in the system and returns access token.
// If user exists, but password is incorrect, returns error.
// If user doesn't exist, returns error.
func (a *Auth) Login(email, password string) (string, error) {
	err := a.email.IsCorrect(email)
	if err != nil {
		return "", err
	}

	user, err := a.storage.User(email)
	if err != nil {
		return "", err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PassHash), []byte(password))
	if err != nil {
		log.Print(err.Error())
		return "", &errors.ErrorWithStatusCode{Message: "Wrong password", StatusCode: http.StatusBadRequest}
	}

	token, err := a.jwt.NewToken(user)
	if err != nil {
		log.Print(err.Error())
		return "", err
	}

	return token, nil
}
