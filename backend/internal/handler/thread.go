package handler

import (
	"log"
	"net/http"

	"strconv"

	"github.com/gorilla/mux"
	"github.com/itchan-dev/itchan/shared/domain"
)

func (h *handler) CreateThread(w http.ResponseWriter, r *http.Request) {
	type bodyJson struct {
		Title       string              `validate:"required" json:"title"`
		Text        string              `validate:"required" json:"text"`
		Attachments []domain.Attachment `validate:"required" json:"attachments"`
	}
	var body bodyJson
	if err := loadAndValidateRequestBody(r, &body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	accessCookie, err := r.Cookie("accessToken")
	if err != nil {
		http.Error(w, "can't get accessToken cookie", http.StatusUnauthorized)
		return
	}
	uid, err := getFieldFromCookie[int64](h, accessCookie, "uid")
	if err != nil {
		http.Error(w, "Cant parse cookie", http.StatusInternalServerError)
		return
	}
	op_msg := domain.Message{Author: domain.User{Id: *uid}, Text: body.Text, Attachments: body.Attachments}

	_, err = h.thread.Create(body.Title, mux.Vars(r)["board"], &op_msg)
	if err != nil {
		http.Error(w, "tmp", http.StatusInternalServerError)
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
