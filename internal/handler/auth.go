package handler

import (
	"encoding/json"

	"log"
	"net/http"
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
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		log.Print(err.Error())
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	accessToken, err := h.auth.Login(creds.Email, creds.Password)
	if err != nil {
		log.Print(err.Error())
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
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
		MaxAge:   0,
		HttpOnly: true,
	}
	http.SetCookie(w, cookie)

	w.WriteHeader(http.StatusOK)
}
