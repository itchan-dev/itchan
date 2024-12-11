package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/itchan-dev/itchan/shared/domain"
)

func (h handler) CreateMessage(w http.ResponseWriter, r *http.Request) {
	type bodyJson struct {
		Title       string              `json:"title"`
		Text        string              `json:"text"`
		Attachments []domain.Attachment `json:"attachments"`
	}
	var body bodyJson
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		log.Print(err.Error())
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	accessCookie, err := r.Cookie("accessToken")
	if err != nil {
		http.Error(w, "can't get accessToken cookie", http.StatusUnauthorized)
		return
	}
	jwtClaims, err := h.jwt.DecodeToken(accessCookie.Value)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	uid, ok := jwtClaims["uid"].(int64)
	if !ok {
		http.Error(w, "Can't find uid in jwtClaims", http.StatusInternalServerError)
		return
	}
	threadIdStr := mux.Vars(r)["thread"]
	threadId, err := strconv.Atoi(threadIdStr)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	_, err = h.message.Create(mux.Vars(r)["board"], &domain.User{Id: uid}, body.Text, body.Attachments, int64(threadId))
	if err != nil {
		log.Print(err.Error())
		http.Error(w, "Bad request", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("Created"))
}

func (h handler) GetMessage(w http.ResponseWriter, r *http.Request) {
	// board := mux.Vars(r)["board"]
	msgIdStr := mux.Vars(r)["message"]
	msgId, err := strconv.Atoi(msgIdStr)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	msg, err := h.message.Get(int64(msgId))
	if err != nil {
		log.Print(err.Error())
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	writeJSON(w, msg)
}

func (h handler) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	board := mux.Vars(r)["board"]
	msgIdStr := mux.Vars(r)["message"]
	msgId, err := strconv.Atoi(msgIdStr)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if err := h.message.Delete(board, int64(msgId)); err != nil {
		log.Print(err.Error())
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("deleted"))
}
