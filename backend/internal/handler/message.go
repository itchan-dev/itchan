package handler

import (
	"errors"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/itchan-dev/itchan/shared/api"
	"github.com/itchan-dev/itchan/shared/domain"
	mw "github.com/itchan-dev/itchan/shared/middleware"
	"github.com/itchan-dev/itchan/shared/utils"
	"github.com/itchan-dev/itchan/shared/validation"
	_ "golang.org/x/image/webp"
)

func (h *Handler) CreateMessage(w http.ResponseWriter, r *http.Request) {
	board := mux.Vars(r)["board"]
	threadIdStr := mux.Vars(r)["thread"]
	threadId, err := parseIntParam(threadIdStr, "thread ID")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	user := mw.GetUserFromContext(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	body, pendingFiles, cleanup, err := parseMultipartRequest[api.CreateMessageRequest](w, r, h)
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

	creation := domain.MessageCreationData{
		Board:        domain.BoardShortName(board),
		ThreadId:     domain.ThreadId(threadId),
		Author:       *user,
		Text:         domain.MsgText(body.Text),
		PendingFiles: pendingFiles,
		ReplyTo:      body.ReplyTo,
	}

	msgId, ordinal, err := h.message.Create(creation)
	if err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	// Calculate page from ordinal dynamically
	page := 1
	if h.cfg.Public.MessagesPerThreadPage > 0 && ordinal > 0 {
		page = (ordinal-1)/h.cfg.Public.MessagesPerThreadPage + 1
	}

	// Return the created message ID and page
	w.WriteHeader(http.StatusCreated)
	response := api.CreateMessageResponse{Id: int64(msgId), Page: page}
	writeJSON(w, response)
}

func (h *Handler) GetMessage(w http.ResponseWriter, r *http.Request) {
	board := mux.Vars(r)["board"]
	msgIdStr := mux.Vars(r)["message"]
	msgId, err := parseIntParam(msgIdStr, "message ID")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	msg, err := h.message.Get(board, domain.MsgId(msgId))
	if err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	response := api.MessageResponse{Message: msg}
	writeJSON(w, response)
}

func (h *Handler) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	board := mux.Vars(r)["board"]
	msgIdStr := mux.Vars(r)["message"]
	msgId, err := parseIntParam(msgIdStr, "message ID")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.message.Delete(board, domain.MsgId(msgId)); err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}
