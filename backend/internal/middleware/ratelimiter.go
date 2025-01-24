package middleware

import (
	"errors"
	"net/http"

	"github.com/itchan-dev/itchan/backend/internal/handler"
	"github.com/itchan-dev/itchan/backend/internal/middleware/ratelimiter"

	"github.com/itchan-dev/itchan/backend/internal/utils"
)

func RateLimit(url *ratelimiter.UserRateLimiter, getIdentity func(r *http.Request) (string, error)) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			identity, err := getIdentity(r)
			if err != nil {
				utils.WriteErrorAndStatusCode(w, err)
			}
			if !url.Allow(identity) {
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		}
	}
}

func GetEmailFromContext(r *http.Request) (string, error) {
	user := GetUserFromContext(r)
	if user == nil {
		return "", errors.New("Can't get user email")
	}
	return user.Email, nil
}

func GetIP(r *http.Request) (string, error) {
	ip, err := utils.GetIP(r)
	if err != nil {
		return "", err
	}
	return ip, nil
}

func GetEmailFromBody(r *http.Request) (string, error) {
	var creds struct {
		Email    string `validate:"required" json:"email"`
		Password string `json:"password"`
	}
	if err := handler.LoadAndValidateRequestBody(r, &creds); err != nil {
		return "", err
	}
	return creds.Email, nil
}

func LimitByIpAndEmail(rl *ratelimiter.UserRateLimiter, handler http.HandlerFunc) http.HandlerFunc {
	return RateLimit(rl, GetIP)(
		RateLimit(rl, GetEmailFromBody)(
			handler))
}
