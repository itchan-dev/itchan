package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/itchan-dev/itchan/shared/domain"
)

func main() {
	r := mux.NewRouter()

	tmpl := template.Must(template.ParseFiles("./frontend/templates/index.html", "./frontend/templates/board.html", "./frontend/templates/thread.html", "./frontend/templates/login.html"))

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		resp, err := http.Get("http://itchan:8080/v1/boards") // Get all boards
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		log.Println(string(body))
		var boards []domain.Board
		if err := json.NewDecoder(resp.Body).Decode(&boards); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		tmpl.ExecuteTemplate(w, "index.html", struct{ Boards []domain.Board }{boards})
	})

	r.HandleFunc("/b/{board}", func(w http.ResponseWriter, r *http.Request) {
		shortName := mux.Vars(r)["board"]

		resp, err := http.Get(fmt.Sprintf("http://itchan:8080/v1/%s", shortName))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		var board domain.Board
		if err := json.NewDecoder(resp.Body).Decode(&board); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Check if Threads is nil (if the board has no threads yet)
		if board.Threads == nil {
			board.Threads = []*domain.Thread{} // Assign an empty slice to avoid nil pointer dereference
		}

		tmpl.ExecuteTemplate(w, "board.html", board)
	})

	r.HandleFunc("/b/{board}/{thread}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		threadID, err := strconv.ParseInt(vars["thread"], 10, 64)
		if err != nil {
			http.Error(w, "Invalid thread ID", http.StatusBadRequest)
			return
		}

		resp, err := http.Get(fmt.Sprintf("http://itchan:8080/v1/%s/%d", vars["board"], threadID))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		var thread domain.Thread
		if err := json.NewDecoder(resp.Body).Decode(&thread); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		tmpl.ExecuteTemplate(w, "thread.html", thread)
	})

	r.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			tmpl.ExecuteTemplate(w, "login.html", nil) // Render the login form
			return
		}

		// Handle POST request (login submission)
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		form := url.Values{}
		form.Set("email", r.Form.Get("email"))
		form.Set("password", r.Form.Get("password"))

		resp, err := http.Post(
			"http://itchan:8080/v1/auth/login", // Your backend login endpoint
			"application/x-www-form-urlencoded",
			strings.NewReader(form.Encode()),
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			// Handle login error (e.g., incorrect credentials)
			http.Error(w, "Login failed", http.StatusUnauthorized)
			return
		}

		for _, cookie := range resp.Cookies() {
			if cookie.Name == "accessToken" { // Check for access token
				http.SetCookie(w, cookie)
				break // Exit the loop once the cookie is found
			}
		}

		// Redirect to main page or previous page after successful login
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	httpPort := os.Getenv("PORT")
	if httpPort == "" {
		httpPort = "8081"
	}
	log.Print("frontend started")
	log.Fatal(http.ListenAndServe(":"+httpPort, r))
}
