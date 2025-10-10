package handler

import (
	"html/template"
	"net/http"

	"github.com/itchan-dev/itchan/frontend/internal/markdown"
	"github.com/itchan-dev/itchan/shared/config"
	// Added for splitAndTrim if not already imported elsewhere
)

type credentials struct {
	Email    string `validate:"required" json:"email"`
	Password string `validate:"required" json:"password"`
}

type Handler struct {
	Templates     map[string]*template.Template
	Public        config.Public
	TextProcessor *markdown.TextProcessor
}

func New(templates map[string]*template.Template, publicCfg config.Public, textProcessor *markdown.TextProcessor) *Handler {
	return &Handler{
		Templates:     templates,
		Public:        publicCfg,
		TextProcessor: textProcessor,
	}
}

func FaviconHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/favicon.ico")
}
