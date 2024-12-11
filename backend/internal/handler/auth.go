package handler

import (
	"database/sql"
	"encoding/json"
	"errors"

	"log"
	"net/http"

	internal_errors "github.com/itchan-dev/itchan/backend/internal/errors"
)

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *handler) Signup(w http.ResponseWriter, r *http.Request) {
	var creds credentials
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		log.Print(err.Error())
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	_, err := h.auth.Signup(creds.Email, creds.Password)
	if err != nil {
		log.Print(err.Error())
		http.Error(w, "Unauthorized", http.StatusUnauthorized) // think about InternalError
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("Created. You can login now"))
}

func (h *handler) Login(w http.ResponseWriter, r *http.Request) {
	var creds credentials
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&creds); err != nil {
		log.Print(err.Error())
		http.Error(w, "Cant parse body", http.StatusBadRequest)
		return
	}
	if creds.Email == "" || creds.Password == "" {
		http.Error(w, "Both 'email' and 'password' should be specified", http.StatusBadRequest)
		return
	}

	accessToken, err := h.auth.Login(creds.Email, creds.Password)
	if err != nil {
		log.Print(err.Error())
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, internal_errors.WrongPassword) {
			http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		} else if internal_errors.Is[*internal_errors.ValidationError](err) {
			http.Error(w, "Invalid email format", http.StatusBadRequest)
		} else {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}
	cookie := &http.Cookie{
		Path:     "/",
		Name:     "accessToken",
		Value:    accessToken,
		MaxAge:   int(h.cfg.JwtTTL().Seconds()),
		HttpOnly: true,
	}
	http.SetCookie(w, cookie)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("You logged in"))
}

func (h handler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie := &http.Cookie{
		Path:     "/",
		Name:     "accessToken",
		Value:    "",
		MaxAge:   -1,
		HttpOnly: true,
	}
	http.SetCookie(w, cookie)

	w.WriteHeader(http.StatusOK)
}
