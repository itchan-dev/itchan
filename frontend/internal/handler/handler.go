package handler

import (
	"html/template"
	"net/http"

	"github.com/itchan-dev/itchan/frontend/internal/apiclient"
	"github.com/itchan-dev/itchan/frontend/internal/markdown"
	"github.com/itchan-dev/itchan/shared/config"
)

type Handler struct {
	Templates     map[string]*template.Template
	Public        config.Public
	TextProcessor *markdown.TextProcessor
	APIClient     *apiclient.APIClient
	MediaPath     string // Exposed for router to create file server
}

func New(templates map[string]*template.Template, publicCfg config.Public, textProcessor *markdown.TextProcessor, apiClient *apiclient.APIClient, mediaPath string) *Handler {
	return &Handler{
		Templates:     templates,
		Public:        publicCfg,
		TextProcessor: textProcessor,
		APIClient:     apiClient,
		MediaPath:     mediaPath,
	}
}

func FaviconHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/favicon.ico")
}
