package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
	"github.com/itchan-dev/itchan/shared/domain"
	mw "github.com/itchan-dev/itchan/shared/middleware"
	"github.com/itchan-dev/itchan/shared/utils"
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

func (h *Handler) renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	tmpl, ok := h.Templates[name]
	if !ok {
		http.Error(w, fmt.Sprintf("Template %s not found", name), http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func FaviconHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/favicon.ico")
}

func getBoards(r *http.Request) ([]domain.Board, error) {
	req, err := requestWithCookie(r, "GET", "http://api:8080/v1/boards", nil, "accessToken")
	if err != nil {
		return nil, errors.New("Internal error: request creation failed")
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf(err.Error())
		return nil, errors.New("Internal error: backend unavailable")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("Internal error: backend status %d", resp.StatusCode))
	}
	var boards []domain.Board
	err = utils.Decode(resp.Body, &boards)
	if err != nil {
		return nil, errors.New("Internal error: cant decode response")
	}
	return boards, nil
}

func (h *Handler) IndexGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		Boards []domain.Board
		Error  template.HTML
		User   *domain.User
	}
	templateData.User = mw.GetUserFromContext(r) // use after auth middleware

	boards, err := getBoards(r)
	if err != nil {
		templateData.Error = template.HTML(err.Error())
		h.renderTemplate(w, "index.html", templateData)
		return
	}
	templateData.Boards = boards

	if errorParam := r.URL.Query().Get("error"); errorParam != "" {
		decodedError, err := url.QueryUnescape(errorParam)
		if err != nil {
			decodedError = "Error occurred"
		}
		templateData.Error = template.HTML(template.HTMLEscapeString(decodedError))
	}
	h.renderTemplate(w, "index.html", templateData)
}

func (h *Handler) IndexPostHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		Boards []domain.Board
		Error  template.HTML
		User   *domain.User
	}
	templateData.User = mw.GetUserFromContext(r) // use after auth middleware

	boards, err := getBoards(r)
	if err != nil {
		templateData.Error = template.HTML(err.Error())
		h.renderTemplate(w, "index.html", templateData)
		return
	}
	templateData.Boards = boards

	shortName := r.FormValue("shortName")
	name := r.FormValue("name")
	var allowedEmails []string
	if allowedEmailsStr := r.FormValue("allowedEmails"); allowedEmailsStr != "" {
		allowedEmails = splitAndTrim(allowedEmailsStr)
	} else {
		allowedEmails = nil
	}

	// Make backend request
	backendData := struct {
		Name          string    `json:"name"`
		ShortName     string    `json:"short_name"`
		AllowedEmails *[]string `json:"allowed_emails,omitempty"`
	}{Name: name, ShortName: shortName, AllowedEmails: &allowedEmails}
	backendDataJson, err := json.Marshal(backendData)
	if err != nil {
		log.Println(err.Error())
		templateData.Error = template.HTML("Internal error: cant encode json")
		h.renderTemplate(w, "index.html", templateData)
		return
	}
	req, err := requestWithCookie(r, "POST", "http://api:8080/v1/admin/boards", bytes.NewBuffer(backendDataJson), "accessToken")
	if err != nil {
		templateData.Error = template.HTML(err.Error())
		h.renderTemplate(w, "index.html", templateData)
		return
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf(err.Error())
		templateData.Error = template.HTML("Internal error: backend unavailable")
		h.renderTemplate(w, "index.html", templateData)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		var errMsg string
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Println(err.Error())
			errMsg = "Internal error: cant decode backend response"
		} else {
			errMsg = string(bodyBytes)
		}
		templateData.Error = template.HTML(errMsg)
		h.renderTemplate(w, "index.html", templateData)
		return
	}

	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *Handler) RegisterGetHandler(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, "register.html", nil)
	return
}

func (h *Handler) RegisterPostHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		Error template.HTML
		User  *domain.User
	}

	email := r.FormValue("email")
	creds := credentials{Email: email, Password: r.FormValue("password")}
	credsJson, err := json.Marshal(creds)
	if err != nil {
		log.Println(err.Error())
		templateData.Error = template.HTML("Internal error: cant encode json")
		h.renderTemplate(w, "register.html", templateData)
		return
	}

	resp, err := http.Post("http://api:8080/v1/auth/register", "application/json", bytes.NewBuffer(credsJson))
	if err != nil {
		log.Printf(err.Error())
		templateData.Error = template.HTML("Internal error: backend unavailable")
		h.renderTemplate(w, "register.html", templateData)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			templateData.Error = template.HTML("Internal error: cant decode backend response")
			h.renderTemplate(w, "register.html", templateData)
			return
		}
		if resp.StatusCode == http.StatusTooEarly {
			templateData.Error = template.HTML(fmt.Sprintf(`<span>%s</span><a href="/check_confirmation_code?email=%s">Confirmation link</a>`, string(bodyBytes), email))
		} else {
			templateData.Error = template.HTML(fmt.Sprintf(`Error: %s`, string(bodyBytes)))
		}

		h.renderTemplate(w, "register.html", templateData)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/check_confirmation_code?email=%s", email), http.StatusSeeOther)
}

func (h *Handler) ConfirmEmailGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		Error            template.HTML
		Success          template.HTML
		EmailPlaceholder string
		User             *domain.User
	}
	templateData.EmailPlaceholder = parseEmail(r)

	h.renderTemplate(w, "check_confirmation_code.html", templateData)
	return
}

func (h *Handler) ConfirmEmailPostHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		Error            template.HTML
		Success          template.HTML
		EmailPlaceholder string
		User             *domain.User
	}
	templateData.EmailPlaceholder = parseEmail(r)

	// Make backend request
	backendData := struct {
		Email            string `json:"email"`
		ConfirmationCode string `json:"confirmation_code"`
	}{Email: r.FormValue("email"), ConfirmationCode: r.FormValue("confirmation_code")}
	backendDataJson, err := json.Marshal(backendData)
	if err != nil {
		log.Println(err.Error())
		templateData.Error = template.HTML("Internal error: cant encode json")
		h.renderTemplate(w, "check_confirmation_code.html", templateData)
		return
	}
	resp, err := http.Post("http://api:8080/v1/auth/check_confirmation_code", "application/json", bytes.NewBuffer(backendDataJson))
	if err != nil {
		log.Printf(err.Error())
		templateData.Error = template.HTML("Internal error: backend unavailable")
		h.renderTemplate(w, "check_confirmation_code.html", templateData)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var errMsg string
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Println(err.Error())
			errMsg = "Internal error: cant decode backend response"
		} else {
			errMsg = string(bodyBytes)
		}
		templateData.Error = template.HTML(errMsg)
		h.renderTemplate(w, "check_confirmation_code.html", templateData)
		return
	}

	templateData.Success = template.HTML(`Success! Login now <a href="/login">login</a>`)
	h.renderTemplate(w, "check_confirmation_code.html", templateData)
}

func (h *Handler) LoginGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		Error            template.HTML
		User             *domain.User
		EmailPlaceholder string
	}
	templateData.EmailPlaceholder = parseEmail(r)

	h.renderTemplate(w, "login.html", templateData)
	return
}

func (h *Handler) LoginPostHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		Error            template.HTML
		User             *domain.User
		EmailPlaceholder string
	}
	templateData.EmailPlaceholder = parseEmail(r)

	// Make backend request
	backendData := struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}{Email: r.FormValue("email"), Password: r.FormValue("password")}
	backendDataJson, err := json.Marshal(backendData)
	if err != nil {
		log.Println(err.Error())
		templateData.Error = template.HTML("Internal error: cant encode json")
		h.renderTemplate(w, "login.html", templateData)
		return
	}
	resp, err := http.Post("http://api:8080/v1/auth/login", "application/json", bytes.NewBuffer(backendDataJson))
	if err != nil {
		log.Printf(err.Error())
		templateData.Error = template.HTML("Internal error: backend unavailable")
		h.renderTemplate(w, "login.html", templateData)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errMsg string
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Println(err.Error())
			errMsg = "Internal error: cant decode backend response"
		} else {
			errMsg = string(bodyBytes)
		}
		templateData.Error = template.HTML(errMsg)
		h.renderTemplate(w, "login.html", templateData)
		return
	}

	// Forward cookies
	for _, cookie := range resp.Cookies() {
		http.SetCookie(w, cookie)
	}

	http.Redirect(w, r, "/", http.StatusFound)
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie := &http.Cookie{
		Path:     "/",
		Name:     "accessToken",
		Value:    "",
		MaxAge:   -1,
		HttpOnly: true,
	}
	http.SetCookie(w, cookie)

	http.Redirect(w, r, "/login", http.StatusFound)
}

func BoardDeleteHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	shortName := vars["board"]

	req, err := requestWithCookie(r, "DELETE", fmt.Sprintf("http://api:8080/v1/admin/%s", shortName), nil, "accessToken")
	if err != nil {
		http.Redirect(w, r, "/?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf(err.Error())
		http.Redirect(w, r, "/?error=backend-unavailable", http.StatusSeeOther)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errMsg string
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Println(err.Error())
			errMsg = "Internal error: cant decode backend response"
		} else {
			errMsg = string(bodyBytes)
		}
		http.Redirect(w, r, "/?error="+url.QueryEscape(errMsg), http.StatusFound)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func getBoardPreview(r *http.Request, shortName string, page string) (*domain.Board, error) {
	req, err := requestWithCookie(r, "GET", fmt.Sprintf("http://api:8080/v1/%s?page=%s", shortName, page), nil, "accessToken")
	if err != nil {
		return nil, errors.New("Internal error: request creation failed")
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf(err.Error())
		return nil, errors.New("Internal error: backend unavailable")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("Internal error: backend status %d", resp.StatusCode))
	}
	var board domain.Board
	err = utils.Decode(resp.Body, &board)
	if err != nil {
		return nil, errors.New("Internal error: cant decode response")
	}
	return &board, nil
}

func (h *Handler) BoardGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		Board *domain.Board
		Error template.HTML
		User  *domain.User
	}
	templateData.User = mw.GetUserFromContext(r) // use after auth middleware
	shortName := mux.Vars(r)["board"]
	page := r.URL.Query().Get("page")

	board, err := getBoardPreview(r, shortName, page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	templateData.Board = board
	if errorParam := r.URL.Query().Get("error"); errorParam != "" {
		decodedError, err := url.QueryUnescape(errorParam)
		if err != nil {
			decodedError = "Error occurred"
		}
		templateData.Error = template.HTML(template.HTMLEscapeString(decodedError))
	}
	h.renderTemplate(w, "board.html", templateData)
}

func (h *Handler) BoardPostHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	shortName := vars["board"]
	endpoint := fmt.Sprintf("/%s", shortName)
	// Make backend request
	backendData := struct {
		Title       string              `validate:"required" json:"title"`
		Text        string              `validate:"required" json:"text"`
		Attachments *domain.Attachments `json:"attachments"`
	}{Title: r.FormValue("title"), Text: r.FormValue("text")}
	backendDataJson, err := json.Marshal(backendData)
	if err != nil {
		log.Println(err.Error())
		http.Redirect(w, r, endpoint+"?error="+url.QueryEscape("Internal error: cant encode json"), http.StatusSeeOther)
		return
	}

	req, err := requestWithCookie(r, "POST", fmt.Sprintf("http://api:8080/v1%s", endpoint), bytes.NewBuffer(backendDataJson), "accessToken")
	if err != nil {
		http.Redirect(w, r, endpoint+"?error="+url.QueryEscape("Internal error: "+err.Error()), http.StatusSeeOther)
		return
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Redirect(w, r, endpoint+"?error="+url.QueryEscape("Internal error: "+err.Error()), http.StatusSeeOther)
		return
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err.Error())
		http.Redirect(w, r, endpoint+"?error="+url.QueryEscape("Internal error: cant decode backend response"), http.StatusSeeOther)
		return
	}
	msg := string(bodyBytes)
	if resp.StatusCode != http.StatusCreated {
		http.Redirect(w, r, endpoint+"?error="+url.QueryEscape("Backend wrong status code: "+msg), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, endpoint+"/"+msg, http.StatusSeeOther)
}

func (h *Handler) ThreadGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		Thread *domain.Thread
		Error  template.HTML
		User   *domain.User
	}
	templateData.User = mw.GetUserFromContext(r) // use after auth middleware
	shortName := mux.Vars(r)["board"]
	threadId := mux.Vars(r)["thread"]

	req, err := requestWithCookie(r, "GET", fmt.Sprintf("http://api:8080/v1/%s/%s", shortName, threadId), nil, "accessToken")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf(err.Error())
		http.Error(w, "Internal error: backend unavailable", http.StatusInternalServerError)
		return
	}
	if resp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("Internal error: backend status %d", resp.StatusCode), http.StatusInternalServerError)
		return
	}
	var thread domain.Thread
	err = utils.Decode(resp.Body, &thread)
	if err != nil {
		log.Printf(err.Error())
		http.Error(w, "Internal error: cant decode response", http.StatusInternalServerError)
		return
	}

	templateData.Thread = &thread
	if errorParam := r.URL.Query().Get("error"); errorParam != "" {
		decodedError, err := url.QueryUnescape(errorParam)
		if err != nil {
			decodedError = "Error occurred"
		}
		templateData.Error = template.HTML(template.HTMLEscapeString(decodedError))
	}
	h.renderTemplate(w, "threadpage.html", templateData)
}

func (h *Handler) ThreadPostHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	shortName := vars["board"]
	threadId := vars["thread"]
	endpoint := fmt.Sprintf("/%s/%s", shortName, threadId)
	// Make backend request
	backendData := struct {
		Text        string              `validate:"required" json:"text"`
		Attachments *domain.Attachments `json:"attachments"`
	}{Text: r.FormValue("text")}
	backendDataJson, err := json.Marshal(backendData)
	if err != nil {
		log.Println(err.Error())
		http.Redirect(w, r, endpoint+"?error="+url.QueryEscape("Internal error: cant encode json"), http.StatusSeeOther)
		return
	}

	req, err := requestWithCookie(r, "POST", fmt.Sprintf("http://api:8080/v1%s", endpoint), bytes.NewBuffer(backendDataJson), "accessToken")
	if err != nil {
		http.Redirect(w, r, endpoint+"?error="+url.QueryEscape("Internal error: "+err.Error()), http.StatusSeeOther)
		return
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Redirect(w, r, endpoint+"?error="+url.QueryEscape("Internal error: "+err.Error()), http.StatusSeeOther)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var errMsg string
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Println(err.Error())
			errMsg = "Internal error: cant decode backend response"
		} else {
			errMsg = string(bodyBytes)
		}
		http.Redirect(w, r, endpoint+"?error="+url.QueryEscape("Backend wrong status code: "+errMsg), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, endpoint, http.StatusSeeOther)
}
