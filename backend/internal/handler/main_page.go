package handler

import (
	"net/http"
)

func (h *Handler) GetMainPage(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Моя домашняя страница!"))
}
