package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/itchan-dev/itchan/shared/api"
	"github.com/itchan-dev/itchan/shared/domain"
	mw "github.com/itchan-dev/itchan/shared/middleware"
	"github.com/itchan-dev/itchan/shared/utils"
)

func (h *Handler) CreateBoard(w http.ResponseWriter, r *http.Request) {
	var body api.CreateBoardRequest
	if err := utils.DecodeValidate(r.Body, &body); err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	err := h.board.Create(domain.BoardCreationData{Name: domain.BoardName(body.Name), ShortName: domain.BoardShortName(body.ShortName), AllowedEmails: body.AllowedEmails})
	if err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (h *Handler) GetBoard(w http.ResponseWriter, r *http.Request) {
	shortName := chi.URLParam(r, "board")
	page := utils.GetPage(r)

	board, err := h.board.Get(shortName, page)
	if err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	writeJSON(w, board)
}

func (h *Handler) GetBoardLastModified(w http.ResponseWriter, r *http.Request) {
	shortName := chi.URLParam(r, "board")

	lastModified, err := h.board.GetLastModified(domain.BoardShortName(shortName))
	if err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	writeJSON(w, api.LastModifiedResponse{LastModifiedAt: lastModified})
}

func (h *Handler) DeleteBoard(w http.ResponseWriter, r *http.Request) {
	shortName := chi.URLParam(r, "board")

	err := h.board.Delete(shortName)
	if err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) GetBoards(w http.ResponseWriter, r *http.Request) {
	user := mw.GetUserFromContext(r)
	if user == nil {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}
	boards, err := h.board.GetAll(*user)
	if err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	boardMetadata := make([]domain.BoardMetadata, len(boards))
	for i, board := range boards {
		boardMetadata[i] = board.BoardMetadata
	}

	writeJSON(w, boardMetadata)
}
