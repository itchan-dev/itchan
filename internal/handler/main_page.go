package handler

import (
	"net/http"
)

func (h handler) GetMainPage(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Моя домашняя страница!"))
}
