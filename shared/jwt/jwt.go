package jwt

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/itchan-dev/itchan/shared/domain"
	internal_errors "github.com/itchan-dev/itchan/shared/errors"
)

type JwtService interface {
	NewToken(user domain.User) (string, error)
	DecodeToken(jwtStr string) (*jwt.Token, error)
}

type Jwt struct {
	secretKey string
	ttl       time.Duration
}

func New(secretKey string, ttl time.Duration) JwtService {
	return &Jwt{secretKey, ttl}
}

func (j *Jwt) NewToken(user domain.User) (string, error) {
	claims := jwt.MapClaims{}
	claims["uid"] = user.Id
	claims["email"] = user.Email
	claims["admin"] = user.Admin
	claims["exp"] = time.Now().Add(j.ttl).Unix()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(j.secretKey))
	if err != nil {
		log.Print(err.Error())
		return "", errors.New("Can't create token")
	}

	return tokenString, nil
}

func (j *Jwt) DecodeToken(jwtStr string) (*jwt.Token, error) {
	token, err := jwt.Parse(jwtStr, func(token *jwt.Token) (interface{}, error) {
		// Verify signing algorithm
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, &internal_errors.ErrorWithStatusCode{Message: fmt.Sprintf("Unexpected signing method: %v", token.Header["alg"]), StatusCode: http.StatusUnauthorized}
		}
		return []byte(j.secretKey), nil
	})
	if err != nil {
		log.Println(err)
		return nil, &internal_errors.ErrorWithStatusCode{Message: "Invalid token signature", StatusCode: http.StatusUnauthorized}
	}

	if !token.Valid {
		return nil, &internal_errors.ErrorWithStatusCode{Message: "Invalid access token", StatusCode: http.StatusUnauthorized}
	}

	return token, nil
}
