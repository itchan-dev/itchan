package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/itchan-dev/itchan/shared/api"
	"github.com/itchan-dev/itchan/shared/domain"
	mw "github.com/itchan-dev/itchan/shared/middleware"
	"github.com/itchan-dev/itchan/shared/utils"
)

const default_page int = 1

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
	pageQuery := r.URL.Query().Get("page")
	var page int
	var err error
	if pageQuery == "" {
		page = default_page
	} else {
		if page, err = parseIntParam(pageQuery, "page"); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	board, err := h.board.Get(shortName, page)
	if err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	response := api.BoardResponse{Board: board}
	writeJSON(w, response)
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

	// Convert to response DTOs (metadata only for list view)
	boardMetadata := make([]api.BoardMetadataResponse, len(boards))
	for i, board := range boards {
		boardMetadata[i] = api.BoardMetadataResponse{BoardMetadata: board.BoardMetadata}
	}

	response := api.BoardListResponse{Boards: boardMetadata}
	writeJSON(w, response)
}
