package handler

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	frontend_domain "github.com/itchan-dev/itchan/frontend/internal/domain"
	"github.com/itchan-dev/itchan/shared/domain"
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

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func (h *Handler) renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	tmpl, ok := h.Templates[name]
	if !ok {
		http.Error(w, fmt.Sprintf("Template %s not found", name), http.StatusInternalServerError)
		return
	}

	// It's good practice to use a buffer to catch template execution errors
	// before writing potentially partial output to the ResponseWriter.
	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, data); err != nil {
		log.Printf("Error executing template %s: %v", name, err)
		http.Error(w, "Internal Server Error rendering template", http.StatusInternalServerError)
		return
	}

	// If execution was successful, write the buffer to the response.
	_, err := buf.WriteTo(w)
	if err != nil {
		// This error is less common, usually related to network issues.
		log.Printf("Error writing template buffer to response: %v", err)
		// The header might have already been written, so we might not be able to send a new error status.
	}
}

// Helper function to redirect with an error message
func redirectWithError(w http.ResponseWriter, r *http.Request, targetURL string, errMsg string) {
	// Ensure the error message is URL encoded
	encodedError := url.QueryEscape(errMsg)
	// Append error parameter correctly, handling existing query parameters
	separator := "?"
	if strings.Contains(targetURL, "?") {
		separator = "&"
	}
	redirectURL := fmt.Sprintf("%s%serror=%s", targetURL, separator, encodedError)
	http.Redirect(w, r, redirectURL, http.StatusSeeOther) // Use StatusSeeOther for PRG
}

// Helper function to redirect with success and possibly other params
func redirectWithSuccess(w http.ResponseWriter, r *http.Request, targetURL string, successParam string) {
	// Append success parameter correctly, handling existing query parameters
	separator := "?"
	if strings.Contains(targetURL, "?") {
		separator = "&"
	}
	redirectURL := fmt.Sprintf("%s%ssuccess=%s", targetURL, separator, url.QueryEscape(successParam))
	http.Redirect(w, r, redirectURL, http.StatusSeeOther) // Use StatusSeeOther for PRG
}

// Helper function to parse error/success from query params for GET handlers
func parseMessagesFromQuery(r *http.Request) (errMsg template.HTML, successMsg template.HTML) {
	if errorParam := r.URL.Query().Get("error"); errorParam != "" {
		decodedError, err := url.QueryUnescape(errorParam)
		if err != nil {
			// Handle decoding error - maybe log it and show a generic message
			log.Printf("Error decoding error query parameter: %v", err)
			decodedError = "An error occurred (failed to decode message)."
		}
		// It's crucial to escape the error message before rendering it as HTML
		// to prevent XSS if the error message contains user input or unexpected characters.
		errMsg = template.HTML(template.HTMLEscapeString(decodedError))
	}

	if successParam := r.URL.Query().Get("success"); successParam != "" {
		decodedSuccess, err := url.QueryUnescape(successParam)
		if err != nil {
			log.Printf("Error decoding success query parameter: %v", err)
			decodedSuccess = "Operation successful (failed to decode message)."
		}
		// Be cautious if success messages can contain arbitrary content.
		// If it's just a flag like "true" or "1", direct rendering might be okay.
		// If it contains generated HTML like links, ensure it's safe or construct it server-side.
		// For now, assume it's potentially unsafe and escape it. If specific HTML is needed,
		// the GET handler should generate it based on the success flag.
		successMsg = template.HTML(template.HTMLEscapeString(decodedSuccess))

		// Example: If successParam == "email_confirmed"
		// if decodedSuccess == "email_confirmed" {
		//  successMsg = template.HTML(`Success! Login now <a href="/login">login</a>`)
		// }
	}
	return errMsg, successMsg
}

var (
	messageLinkRegex = regexp.MustCompile(`>>(\d+)/(\d+)`) // Use > because text is likely HTML escaped
)

// processMessageLinks finds >>N patterns and converts them to internal links
// with attributes for the preview functionality. It returns template.HTML
// to prevent double-escaping in the template.
func processMessageLinks(message domain.Message) template.HTML {
	// The input 'text' might already be HTML escaped by a markdown processor or similar.
	// We specifically look for '>>' followed by digits.
	// ReplaceAllStringFunc allows custom replacement logic for each match.
	processedText := messageLinkRegex.ReplaceAllStringFunc(message.Text, func(match string) string {
		// Extract the numeric ID part from the match (e.g., "123" from ">>123")
		idStr := match[len(">>"):]

		// Create the link HTML
		// - href points to the anchor #p<ID> on the current page.
		// - class="message-link" identifies it for JS hover listeners.
		// - data-message-id stores the ID for the JS to fetch the preview.
		return fmt.Sprintf(`<a href="/%[2]s/%[3]d#p%[1]s" class="message-link" data-message-id="%[1]s">>>%[1]s</a>`,
			idStr, message.Board, message.ThreadId)
	})

	return template.HTML(processedText)
}

func RenderMessage(message domain.Message) frontend_domain.Message {
	renderedMessage := frontend_domain.Message{Message: message}
	renderedMessage.Text = processMessageLinks(message)
	return renderedMessage
}

// render all messages and proccess links
func RenderThread(thread domain.Thread) frontend_domain.Thread {
	renderedThread := frontend_domain.Thread{ThreadMetadata: thread.ThreadMetadata, Messages: make([]frontend_domain.Message, len(thread.Messages))}
	for i, msg := range thread.Messages {
		renderedThread.Messages[i] = RenderMessage(msg)
	}
	return renderedThread
}

func RenderBoard(board domain.Board) frontend_domain.Board {
	renderedBoard := frontend_domain.Board{BoardMetadata: board.BoardMetadata, Threads: make([]frontend_domain.Thread, len(board.Threads))}
	for i, thread := range board.Threads {
		renderedBoard.Threads[i] = RenderThread(thread)
	}
	return renderedBoard
}
