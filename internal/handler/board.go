package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

func (h *handler) CreateBoard(w http.ResponseWriter, r *http.Request) {
	type bodyJson struct {
		Name      string `json:"name"`
		ShortName string `json:"short_name"`
	}
	var body bodyJson
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		log.Print(err.Error())
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	err := h.board.Create(body.Name, body.ShortName)
	if err != nil {
		log.Print(err.Error())
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("created"))
}

func (h *handler) GetBoard(w http.ResponseWriter, r *http.Request) {
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil {
		log.Print(err.Error())
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	shortName := mux.Vars(r)["board"]

	board, err := h.board.Get(shortName, page)
	if err != nil {
		log.Print(err.Error())
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	writeJSON(w, board)
}

func (h *handler) DeleteBoard(w http.ResponseWriter, r *http.Request) {
	shortName := mux.Vars(r)["board"]

	err := h.board.Delete(shortName)
	if err != nil {
		log.Print(err.Error())
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("deleted"))
}
