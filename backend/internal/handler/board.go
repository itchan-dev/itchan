package handler

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
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
	shortName := mux.Vars(r)["board"]
	pageQuery := r.URL.Query().Get("page")
	var page int
	var err error
	if pageQuery == "" {
		page = default_page
	} else {
		if page, err = strconv.Atoi(pageQuery); err != nil {
			log.Print(err.Error())
			http.Error(w, "Page param should be integer", http.StatusBadRequest)
			return
		}
	}

	board, err := h.board.Get(shortName, page)
	if err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	writeJSON(w, board)
}

func (h *Handler) DeleteBoard(w http.ResponseWriter, r *http.Request) {
	shortName := mux.Vars(r)["board"]

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

	writeJSON(w, boards)
}
