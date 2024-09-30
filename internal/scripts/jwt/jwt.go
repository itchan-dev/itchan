package jwt

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/itchan-dev/itchan/internal/domain"
)

type Jwt struct {
	secretKey string
	ttl       time.Duration
}

func New(secretKey string, ttl time.Duration) *Jwt {
	return &Jwt{secretKey, ttl}
}

func (j *Jwt) NewToken(user *domain.User) (string, error) {
	claims := jwt.MapClaims{}
	claims["uid"] = user.Id
	claims["email"] = user.Email
	claims["admin"] = user.Admin
	claims["exp"] = time.Now().Add(j.ttl).Unix()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(j.secretKey))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func (j *Jwt) DecodeToken(jwtStr string) (jwt.MapClaims, error) {
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(jwtStr, &claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(j.secretKey), nil
	})
	if err != nil {
		return claims, err
	}
	return claims, nil
}

// func IsExpired(jwtToken, jwtKey string) error {
// 	claims := jwt.MapClaims{}
// 	_, err := jwt.ParseWithClaims(jwtToken, &claims, func(token *jwt.Token) (interface{}, error) {
// 		return []byte(jwtKey), nil
// 	})
// 	if err != nil {
// 		return err
// 	}
// 	expTime, success := claims["exp"].(int64)
// 	if success {
// 		return fmt.Errorf("error parsing accessToken")
// 	}
// 	if time.Now().Unix() < expTime {
// 		return fmt.Errorf("accessToken expired")
// 	}
// 	return nil
// }
