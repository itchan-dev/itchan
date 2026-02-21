package handler

import (
	"net/http"

	frontend_domain "github.com/itchan-dev/itchan/frontend/internal/domain"
	"github.com/itchan-dev/itchan/shared/api"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/itchan-dev/itchan/shared/logger"
	mw "github.com/itchan-dev/itchan/shared/middleware"
)

// AccountGetHandler displays the user's account page with activity
func (h *Handler) AccountGetHandler(w http.ResponseWriter, r *http.Request) {
	user := mw.GetUserFromContext(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	activity, err := h.APIClient.GetUserActivity(r)
	var errMsg string
	if err != nil {
		logger.Log.Error("failed to get user activity from API", "error", err)
		errMsg = "Failed to load activity"
		activity = &api.UserActivityResponse{
			Messages: []domain.Message{},
		}
	}

	activityMessages := make([]*frontend_domain.Message, len(activity.Messages))
	for i, msg := range activity.Messages {
		activityMessages[i] = renderMessage(msg)
	}

	h.renderTemplateWithError(w, r, "account.html", frontend_domain.AccountPageData{
		ActivityMessages: activityMessages,
	}, errMsg)
}
