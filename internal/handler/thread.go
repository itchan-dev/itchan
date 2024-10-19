package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"strconv"

	"github.com/gorilla/mux"
	"github.com/itchan-dev/itchan/internal/domain"
)

func (h *handler) CreateThread(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	op_msg := domain.Message{Author: domain.User{Id: uid}, Text: body.Text, Attachments: body.Attachments}
	_, err = h.thread.Create(body.Title, mux.Vars(r)["board"], &op_msg)
	if err != nil {
		log.Print(err.Error())
		http.Error(w, "Bad request", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("Created"))
}

func (h *handler) GetThread(w http.ResponseWriter, r *http.Request) {
	// board := mux.Vars(r)["board"]
	threadIdStr := mux.Vars(r)["thread"]
	threadId, err := strconv.Atoi(threadIdStr)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	thread, err := h.thread.Get(int64(threadId))
	if err != nil {
		log.Print(err.Error())
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	writeJSON(w, thread)
}

func (h *handler) DeleteThread(w http.ResponseWriter, r *http.Request) {
	board := mux.Vars(r)["board"]
	threadIdStr := mux.Vars(r)["thread"]
	threadId, err := strconv.Atoi(threadIdStr)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if err := h.thread.Delete(board, int64(threadId)); err != nil {
		log.Print(err.Error())
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("deleted"))
}
