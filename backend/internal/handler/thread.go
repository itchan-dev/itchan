package handler

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/itchan-dev/itchan/shared/api"
	"github.com/itchan-dev/itchan/shared/domain"
	mw "github.com/itchan-dev/itchan/shared/middleware"
	"github.com/itchan-dev/itchan/shared/utils"
	"github.com/itchan-dev/itchan/shared/validation"
)

func (h *Handler) CreateThread(w http.ResponseWriter, r *http.Request) {
	board := mux.Vars(r)["board"]
	user := mw.GetUserFromContext(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	body, pendingFiles, cleanup, err := parseMultipartRequest[api.CreateThreadRequest](w, r, h)
	if err != nil {
		// Return 413 Payload Too Large for size errors, 400 for other errors
		statusCode := http.StatusBadRequest
		if errors.Is(err, validation.ErrPayloadTooLarge) {
			statusCode = http.StatusRequestEntityTooLarge
		}
		http.Error(w, err.Error(), statusCode)
		return
	}
	defer cleanup()

	creation := domain.ThreadCreationData{
		Title:    domain.ThreadTitle(body.Title),
		Board:    board,
		IsSticky: body.IsSticky,
		OpMessage: domain.MessageCreationData{
			Author:       *user,
			Text:         domain.MsgText(body.OpMessage.Text),
			PendingFiles: pendingFiles,
			ReplyTo:      body.OpMessage.ReplyTo,
		},
	}

	threadId, _, err := h.thread.Create(creation)
	if err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, "%d", threadId)
}

func (h *Handler) GetThread(w http.ResponseWriter, r *http.Request) {
	board := mux.Vars(r)["board"]
	threadIdStr := mux.Vars(r)["thread"]
	threadId, err := parseIntParam(threadIdStr, "thread ID")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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
	threadId, err := parseIntParam(threadIdStr, "thread ID")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.thread.Delete(board, int64(threadId)); err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}
