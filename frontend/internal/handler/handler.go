package handler

import (
	"html/template"
	"net/http"
	// Added for splitAndTrim if not already imported elsewhere
)

type credentials struct {
	Email    string `validate:"required" json:"email"`
	Password string `validate:"required" json:"password"`
}

type Handler struct {
	Templates map[string]*template.Template
	Public    struct {
		ThreadTitleMaxLen    int
		MessageTextMaxLen    int
		ConfirmationCodeLen  int
		BoardNameMaxLen      int
		BoardShortNameMaxLen int
	}
}

func New(templates map[string]*template.Template, publicCfg struct {
	ThreadTitleMaxLen    int
	MessageTextMaxLen    int
	ConfirmationCodeLen  int
	BoardNameMaxLen      int
	BoardShortNameMaxLen int
}) *Handler {
	return &Handler{
		Templates: templates,
		Public: struct {
			ThreadTitleMaxLen    int
			MessageTextMaxLen    int
			ConfirmationCodeLen  int
			BoardNameMaxLen      int
			BoardShortNameMaxLen int
		}{
			ThreadTitleMaxLen:    publicCfg.ThreadTitleMaxLen,
			MessageTextMaxLen:    publicCfg.MessageTextMaxLen,
			ConfirmationCodeLen:  publicCfg.ConfirmationCodeLen,
			BoardNameMaxLen:      publicCfg.BoardNameMaxLen,
			BoardShortNameMaxLen: publicCfg.BoardShortNameMaxLen,
		},
	}
}

func FaviconHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/favicon.ico")
}
