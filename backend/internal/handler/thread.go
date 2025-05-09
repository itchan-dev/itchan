package handler

import (
	"fmt"
	"log"
	"net/http"

	"strconv"

	"github.com/gorilla/mux"
	"github.com/itchan-dev/itchan/shared/domain"
	mw "github.com/itchan-dev/itchan/shared/middleware"
	"github.com/itchan-dev/itchan/shared/utils"
)

func (h *Handler) CreateThread(w http.ResponseWriter, r *http.Request) {
	type bodyJson struct {
		Title       string              `validate:"required" json:"title"`
		Text        string              `validate:"required" json:"text"`
		Attachments *domain.Attachments `json:"attachments"`
	}
	var body bodyJson
	if err := utils.DecodeValidate(r.Body, &body); err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}
	user := mw.GetUserFromContext(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	opMsg := domain.MessageCreationData{Author: *user, Text: body.Text, Attachments: body.Attachments}

	id, err := h.thread.Create(domain.ThreadCreationData{Title: body.Title, Board: mux.Vars(r)["board"], OpMessage: opMsg})
	if err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(fmt.Sprintf("%d", id)))
}

func (h *Handler) GetThread(w http.ResponseWriter, r *http.Request) {
	board := mux.Vars(r)["board"]
	threadIdStr := mux.Vars(r)["thread"]
	threadId, err := strconv.Atoi(threadIdStr)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	thread, err := h.thread.Get(board, domain.ThreadId(threadId))
	if err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	writeJSON(w, thread)
}

func (h *Handler) DeleteThread(w http.ResponseWriter, r *http.Request) {
	board := mux.Vars(r)["board"]
	threadIdStr := mux.Vars(r)["thread"]
	threadId, err := strconv.Atoi(threadIdStr)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if err := h.thread.Delete(board, int64(threadId)); err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}
