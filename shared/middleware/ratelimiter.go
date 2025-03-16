package middleware

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/itchan-dev/itchan/shared/middleware/ratelimiter"

	"github.com/itchan-dev/itchan/shared/utils"
)

func RateLimit(rl *ratelimiter.UserRateLimiter, getIdentity func(r *http.Request) (string, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, err := getIdentity(r)
			if err != nil {
				utils.WriteErrorAndStatusCode(w, err)
				return
			}
			if !rl.Allow(identity) {
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func GlobalRateLimit(rl *ratelimiter.UserRateLimiter) func(http.Handler) http.Handler {
	return RateLimit(rl, func(r *http.Request) (string, error) { return "global", nil })
}

// Possible if user was authorized with previous middleware
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
		// Create consistent fingerprint from request characteristics
		fingerprint := fmt.Sprintf("%s|%s|%s",
			r.UserAgent(),
			r.Header.Get("Accept-Language"),
			r.Header.Get("Accept-Encoding"),
		)
		return utils.HashSHA256(fingerprint), nil
	}
	return ip, nil
}

// func GetEmailFromBody(r *http.Request) (string, error) {
// 	var creds struct {
// 		Email    string `validate:"required" json:"email"`
// 		Password string `json:"password"`
// 	}
// 	if err := utils.LoadAndValidateRequestBody(r, &creds); err != nil {
// 		return "", err
// 	}
// 	return creds.Email, nil
// }
