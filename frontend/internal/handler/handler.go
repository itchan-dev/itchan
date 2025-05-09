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
}

func New(templates map[string]*template.Template) *Handler {
	return &Handler{
		Templates: templates,
	}
}

func FaviconHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/favicon.ico")
}
