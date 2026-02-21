package handler

import (
	"html/template"
	"net/http"
	"sync"

	"github.com/itchan-dev/itchan/frontend/internal/apiclient"
	"github.com/itchan-dev/itchan/frontend/internal/markdown"
	"github.com/itchan-dev/itchan/shared/config"
)

type Handler struct {
	mu            sync.RWMutex
	templates     map[string]*template.Template
	Public        config.Public
	TextProcessor *markdown.TextProcessor
	APIClient     *apiclient.APIClient
	MediaPath     string // Exposed for router to create file server
}

func New(templates map[string]*template.Template, publicCfg config.Public, textProcessor *markdown.TextProcessor, apiClient *apiclient.APIClient, mediaPath string) *Handler {
	return &Handler{
		templates:     templates,
		Public:        publicCfg,
		TextProcessor: textProcessor,
		APIClient:     apiClient,
		MediaPath:     mediaPath,
	}
}

// UpdateTemplates atomically replaces the template map. Safe for concurrent use.
func (h *Handler) UpdateTemplates(t map[string]*template.Template) {
	h.mu.Lock()
	h.templates = t
	h.mu.Unlock()
}

// getTemplate looks up a template by name. Safe for concurrent use.
func (h *Handler) getTemplate(name string) (*template.Template, bool) {
	h.mu.RLock()
	tmpl, ok := h.templates[name]
	h.mu.RUnlock()
	return tmpl, ok
}

func FaviconHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/favicon.ico")
}
