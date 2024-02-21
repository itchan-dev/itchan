package handler

import (
	"net/http"
)

func (h handler) GetThread(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("GetThread"))
}

func (h handler) CreateThread(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("CreateThread"))
}

func (h handler) DeleteThread(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("DeleteThread"))
}
