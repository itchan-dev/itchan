package handler

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

const default_page int = 1

func (h *handler) CreateBoard(w http.ResponseWriter, r *http.Request) {
	type bodyJson struct {
		Name      string `validate:"required" json:"name"`
		ShortName string `validate:"required" json:"short_name"`
	}
	var body bodyJson
	if err := loadAndValidateRequestBody(r, &body); err != nil {
		writeErrorAndStatusCode(w, err)
		return
	}

	err := h.board.Create(body.Name, body.ShortName)
	if err != nil {
		writeErrorAndStatusCode(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (h *handler) GetBoard(w http.ResponseWriter, r *http.Request) {
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
		writeErrorAndStatusCode(w, err)
		return
	}

	writeJSON(w, board)
}

func (h *handler) DeleteBoard(w http.ResponseWriter, r *http.Request) {
	shortName := mux.Vars(r)["board"]

	err := h.board.Delete(shortName)
	if err != nil {
		writeErrorAndStatusCode(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}
