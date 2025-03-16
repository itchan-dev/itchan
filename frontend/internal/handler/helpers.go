package handler

import (
	"io"
	"log"
	"net/http"
	"strings"
)

func copyCookies(dst *http.Request, src *http.Request) {
	for _, c := range src.Cookies() {
		dst.AddCookie(c)
	}
}

func requestWithCookie(r *http.Request, method, url string, body io.Reader, cookieName string) (*http.Request, error) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		log.Println("Missing cookie:", err.Error())
		return nil, err
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}
	req.AddCookie(cookie)
	req.Header.Set("Content-Type", "application/json")
	return req, nil
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

func splitAndTrim(input string) []string {
	parts := strings.Split(input, ",")
	var result []string
	for _, part := range parts {
		result = append(result, strings.TrimSpace(part))
	}
	return result
}
