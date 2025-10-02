package handler

import (
	"html/template"
	"net/http"

	"github.com/itchan-dev/itchan/shared/config"
	// Added for splitAndTrim if not already imported elsewhere
)

type credentials struct {
	Email    string `validate:"required" json:"email"`
	Password string `validate:"required" json:"password"`
}

type Handler struct {
	Templates map[string]*template.Template
	Public    config.Public
}

func New(templates map[string]*template.Template, publicCfg config.Public) *Handler {
	return &Handler{
		Templates: templates,
		Public:    publicCfg,
	}
}

func FaviconHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/favicon.ico")
}
