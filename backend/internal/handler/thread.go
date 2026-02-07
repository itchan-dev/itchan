package handler

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/itchan-dev/itchan/shared/api"
	"github.com/itchan-dev/itchan/shared/domain"
	mw "github.com/itchan-dev/itchan/shared/middleware"
	"github.com/itchan-dev/itchan/shared/utils"
	"github.com/itchan-dev/itchan/shared/validation"
)

func (h *Handler) CreateThread(w http.ResponseWriter, r *http.Request) {
	board := chi.URLParam(r, "board")
	user := mw.GetUserFromContext(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	body, pendingFiles, cleanup, err := parseMultipartRequest[api.CreateThreadRequest](w, r, h)
	if err != nil {
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
		IsPinned: body.IsPinned,
		OpMessage: domain.MessageCreationData{
			Author:       *user,
			Text:         domain.MsgText(body.OpMessage.Text),
			PendingFiles: pendingFiles,
			ReplyTo:      body.OpMessage.ReplyTo,
		},
	}

	threadId, err := h.thread.Create(creation)
	if err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, "%d", threadId)
}

func (h *Handler) GetThread(w http.ResponseWriter, r *http.Request) {
	board := chi.URLParam(r, "board")
	threadIdStr := chi.URLParam(r, "thread")
	threadId, err := parseIntParam(threadIdStr, "thread ID")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if parsedPage, err := parseIntParam(pageStr, "page"); err == nil && parsedPage > 0 {
			page = parsedPage
		}
	}

	thread, err := h.thread.Get(board, domain.ThreadId(threadId), page)
	if err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	response := api.ThreadResponse{Thread: thread}
	writeJSON(w, response)
}

func (h *Handler) DeleteThread(w http.ResponseWriter, r *http.Request) {
	board := chi.URLParam(r, "board")
	threadIdStr := chi.URLParam(r, "thread")
	threadId, err := parseIntParam(threadIdStr, "thread ID")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.thread.Delete(board, domain.ThreadId(threadId)); err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) TogglePinnedThread(w http.ResponseWriter, r *http.Request) {
	board := chi.URLParam(r, "board")
	threadIdStr := chi.URLParam(r, "thread")
	threadId, err := parseIntParam(threadIdStr, "thread ID")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	newStatus, err := h.thread.TogglePinned(board, domain.ThreadId(threadId))
	if err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"is_pinned": %t}`, newStatus)
}
