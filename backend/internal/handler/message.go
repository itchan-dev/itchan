package handler

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/itchan-dev/itchan/shared/domain"
	mw "github.com/itchan-dev/itchan/shared/middleware"
	"github.com/itchan-dev/itchan/shared/utils"
)

func (h *Handler) CreateMessage(w http.ResponseWriter, r *http.Request) {
	board := mux.Vars(r)["board"]
	threadIdStr := mux.Vars(r)["thread"]
	threadId, err := strconv.Atoi(threadIdStr)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	type bodyJson struct {
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

	_, err = h.message.Create(board, user, body.Text, body.Attachments, int64(threadId))
	if err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (h *Handler) GetMessage(w http.ResponseWriter, r *http.Request) {
	// board := mux.Vars(r)["board"]
	msgIdStr := mux.Vars(r)["message"]
	msgId, err := strconv.Atoi(msgIdStr)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	msg, err := h.message.Get(int64(msgId))
	if err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	writeJSON(w, msg)
}

func (h *Handler) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	board := mux.Vars(r)["board"]
	msgIdStr := mux.Vars(r)["message"]
	msgId, err := strconv.Atoi(msgIdStr)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if err := h.message.Delete(board, int64(msgId)); err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}
