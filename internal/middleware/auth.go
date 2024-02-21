package middleware

import (
	"net/http"

	"github.com/itchan-dev/itchan/internal/config"
	"github.com/itchan-dev/itchan/internal/scripts/jwt"
)

type Auth struct {
	handler http.Handler
	cfg     config.Config
	jwt     jwt.Jwt
}

func (a *Auth) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	accessCookine, err := r.Cookie("accessToken")
	if err != nil {
		http.Error(w, "please sign-in", http.StatusUnauthorized)
		return
	}
	_, err = a.jwt.DecodeToken(accessCookine.Value)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	a.handler.ServeHTTP(w, r)
}

func NewAuth(handlerToWrap http.Handler, cfg config.Config, jwt jwt.Jwt) *Auth {
	return &Auth{handlerToWrap, cfg, jwt}
}
