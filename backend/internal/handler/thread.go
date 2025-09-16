package handler

import (
	"fmt"
	"log"
	"net/http"

	"strconv"

	"github.com/gorilla/mux"
	"github.com/itchan-dev/itchan/shared/api"
	"github.com/itchan-dev/itchan/shared/domain"
	mw "github.com/itchan-dev/itchan/shared/middleware"
	"github.com/itchan-dev/itchan/shared/utils"
)

func (h *Handler) CreateThread(w http.ResponseWriter, r *http.Request) {
	var body api.CreateThreadRequest
	if err := utils.DecodeValidate(r.Body, &body); err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}
	user := mw.GetUserFromContext(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	creation := domain.ThreadCreationData{
		Title:    domain.ThreadTitle(body.Title),
		Board:    domain.BoardShortName(mux.Vars(r)["board"]),
		IsSticky: body.IsSticky,
		OpMessage: domain.MessageCreationData{
			Author:      *user,
			Text:        domain.MsgText(body.OpMessage.Text),
			Attachments: body.OpMessage.Attachments,
			ReplyTo:     body.OpMessage.ReplyTo,
		},
	}

	id, err := h.thread.Create(creation)
	if err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, "%d", id)
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
