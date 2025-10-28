package handler

import (
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"
)

// redirectWithParams correctly redirects to a target URL after adding the given
// query parameters. It safely handles existing query params and URL fragments (anchors).
func redirectWithParams(w http.ResponseWriter, r *http.Request, targetURL string, params map[string]string) {
	// 1. Parse the base URL to separate path, query, and fragment.
	u, err := url.Parse(targetURL)
	if err != nil {
		// If parsing fails, it's a server-side programming error.
		// Log it and fall back to a simple redirect to a safe URL.
		log.Printf("ERROR: Failed to parse redirect URL '%s': %v", targetURL, err)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// 2. Get the existing query values from the URL.
	query := u.Query()

	// 3. Add the new parameters, overwriting any existing keys.
	for key, value := range params {
		query.Set(key, value)
	}

	// 4. Encode the modified query and assign it back to the URL object.
	u.RawQuery = query.Encode()

	// 5. Perform the redirect using the reassembled URL string.
	// The u.String() method correctly combines the path, new query, and original fragment.
	http.Redirect(w, r, u.String(), http.StatusSeeOther)
}

// parseMessagesFromQuery extracts and decodes error/success messages from URL query parameters.
func parseMessagesFromQuery(r *http.Request) (errMsg template.HTML, successMsg template.HTML) {
	if errorParam, err := url.QueryUnescape(r.URL.Query().Get("error")); err == nil && errorParam != "" {
		errMsg = template.HTML(template.HTMLEscapeString(errorParam))
	}

	if successParam, err := url.QueryUnescape(r.URL.Query().Get("success")); err == nil && successParam != "" {
		successMsg = template.HTML(template.HTMLEscapeString(successParam))
	}
	return
}

// splitAndTrim splits a comma-separated string into a slice of trimmed strings.
func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func parseEmail(r *http.Request) string {
	// set default email value from querystring or form value
	var email string
	if r.URL.Query().Get("email") != "" {
		email = r.URL.Query().Get("email")
	} else {
		email = r.FormValue("email")
	}
	return email
}
