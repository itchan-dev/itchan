package handler

import (
	"html/template"
	"net/http"

	"github.com/itchan-dev/itchan/shared/api"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/itchan-dev/itchan/shared/logger"

	frontend_domain "github.com/itchan-dev/itchan/frontend/internal/domain"
)

// AccountGetHandler displays the user's account page with activity
func (h *Handler) AccountGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		CommonTemplateData
		Activity         *api.UserActivityResponse
		ActivityMessages []*frontend_domain.Message
	}
	templateData.CommonTemplateData = h.InitCommonTemplateData(w, r)

	// Check authentication
	if templateData.User == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Fetch activity from API
	activity, err := h.APIClient.GetUserActivity(r)
	if err != nil {
		logger.Log.Error("failed to get user activity from API", "error", err)
		templateData.Error = template.HTML(template.HTMLEscapeString("Failed to load activity"))
		// Use empty activity on error
		activity = &api.UserActivityResponse{
			Messages: []domain.Message{},
		}
	}
	templateData.Activity = activity

	// Convert messages to frontend MessageView using existing RenderMessage
	templateData.ActivityMessages = make([]*frontend_domain.Message, len(activity.Messages))
	for i, msg := range activity.Messages {
		templateData.ActivityMessages[i] = RenderMessage(msg)
	}

	// Render template
	h.renderTemplate(w, "account.html", templateData)
}
