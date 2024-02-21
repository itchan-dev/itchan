package handler

import (
	"net/http"
)

func (h handler) GetBoard(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("GetBoard"))
}

func (h handler) CreateBoard(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("CreateBoard"))
}

func (h handler) DeleteBoard(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("DeleteBoard"))
}
