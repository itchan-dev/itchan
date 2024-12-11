package service

import (
	"github.com/itchan-dev/itchan/backend/internal/errors"
	"github.com/itchan-dev/itchan/shared/domain"
	"golang.org/x/crypto/bcrypt"
)

type AuthService interface {
	Signup(email, password string) (int64, error)
	Login(email, password string) (string, error)
}

type Auth struct {
	storage AuthStorage
	email   Email
	jwt     Jwt
}

type AuthStorage interface {
	SaveUser(email string, passHash []byte) (int64, error)
	User(email string) (*domain.User, error)
	DeleteUser(email string) error
}

type Email interface {
	Confirm(email string) error
	IsCorrect(email string) error
}

type Jwt interface {
	NewToken(user *domain.User) (string, error)
}

func NewAuth(storage AuthStorage, email Email, jwt Jwt) *Auth {
	return &Auth{storage, email, jwt}
}

// Signup registers new user in the system and returns user ID.
// If user with given username already exists, returns error.
func (a *Auth) Signup(email, password string) (int64, error) {
	var err error

	err = a.email.IsCorrect(email)
	if err != nil {
		return 0, err
	}

	err = a.email.Confirm(email)
	if err != nil {
		return 0, err
	}

	passHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}

	uid, err := a.storage.SaveUser(email, passHash)
	if err != nil {
		return 0, err
	}

	return uid, nil
}

// Login checks if user with given credentials exists in the system and returns access token.
// If user exists, but password is incorrect, returns error.
// If user doesn't exist, returns error.
func (a *Auth) Login(email, password string) (string, error) {
	var err error

	err = a.email.IsCorrect(email)
	if err != nil {
		return "", err
	}

	user, err := a.storage.User(email)
	if err != nil {
		return "", err
	}

	err = bcrypt.CompareHashAndPassword(user.PassHash, []byte(password))
	if err != nil {
		return "", errors.WrongPassword
	}

	token, err := a.jwt.NewToken(user)
	if err != nil {
		return "", err
	}

	return token, nil
}

// // Logout deletes user with given email from the system.
// //
// // If user exists, but password is incorrect, returns error.
// // If user doesn't exist, returns error.
// func (a *Auth) Logout(email, password string) error {
// 	var err error

// 	err = a.email.IsCorrect(email)
// 	if err != nil {
// 		return err
// 	}

// 	user, err := a.storage.User(email)
// 	if err != nil {
// 		return err
// 	}

// 	err = bcrypt.CompareHashAndPassword(user.PassHash, []byte(password))
// 	if err != nil {
// 		return err
// 	}

// 	return a.storage.DeleteUser(email)
// }
