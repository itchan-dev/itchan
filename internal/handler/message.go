package handler

import (
	"net/http"
)

func (h handler) GetMessage(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("GetMessage"))
}

func (h handler) CreateMessage(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("CreateMessage"))
}

func (h handler) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("DeleteMessage"))
}
