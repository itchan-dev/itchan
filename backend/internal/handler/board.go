package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	internal_errors "github.com/itchan-dev/itchan/backend/internal/errors"

	"github.com/gorilla/mux"
)

func (h *handler) CreateBoard(w http.ResponseWriter, r *http.Request) {
	type bodyJson struct {
		Name      string `json:"name"`
		ShortName string `json:"short_name"`
	}
	var body bodyJson
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		log.Print(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if body.Name == "" || body.ShortName == "" {
		http.Error(w, "Both 'name' and 'short_name' should be specified", http.StatusBadRequest)
		return
	}
	err := h.board.Create(body.Name, body.ShortName)
	if err != nil {
		log.Print(err.Error())
		if internal_errors.Is[*internal_errors.ValidationError](err) {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (h *handler) GetBoard(w http.ResponseWriter, r *http.Request) {
	shortName := mux.Vars(r)["board"]
	pageQuery := r.URL.Query().Get("page")
	if pageQuery == "" {
		pageQuery = "1"
	}
	page, err := strconv.Atoi(pageQuery)
	if err != nil {
		log.Print(err.Error())
		http.Error(w, "Page param should be integer", http.StatusBadRequest)
		return
	}
	page = max(1, page)

	board, err := h.board.Get(shortName, page)
	if err != nil {
		log.Print(err.Error())
		if errors.Is(err, internal_errors.NotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else if internal_errors.Is[*internal_errors.ValidationError](err) {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	writeJSON(w, board)
}

func (h *handler) DeleteBoard(w http.ResponseWriter, r *http.Request) {
	shortName := mux.Vars(r)["board"]

	err := h.board.Delete(shortName)
	if err != nil {
		log.Print(err.Error())
		if errors.Is(err, internal_errors.NotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else if internal_errors.Is[*internal_errors.ValidationError](err) {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}
