package handler

import "net/http"

func (h *Handler) ToggleDisableMedia(w http.ResponseWriter, r *http.Request) {
	value := "1"
	maxAge := 365 * 24 * 60 * 60 // 1 year

	if c, err := r.Cookie("disable_media"); err == nil && c.Value == "1" {
		value = ""
		maxAge = -1
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "disable_media",
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		Secure:   h.Public.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	})

	target := r.Referer()
	if target == "" {
		target = "/"
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}
