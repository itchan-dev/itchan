package setup

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/itchan-dev/itchan/frontend/internal/handler"
	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/jwt"
)

const (
	baseTemplate           = "base.html"
	tmplPath               = "templates"
	templateReloadInterval = 5 * time.Second
)

type Dependencies struct {
	Handler *handler.Handler
	Jwt     jwt.JwtService
	Public  config.Public
}

func SetupDependencies() *Dependencies {
	templates := mustLoadTemplates(tmplPath)
	public := fetchPublicConfig()
	h := handler.New(templates, public)
	startTemplateReloader(h, tmplPath)
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}
	jwtSvc := jwt.New(jwtSecret, 2629800000000000) // 1 month expiration

	return &Dependencies{Handler: h, Jwt: jwtSvc, Public: public}
}

func sub(a, b int) int { return a - b }
func add(a, b int) int { return a + b }

func dict(values ...interface{}) (map[string]interface{}, error) {
	if len(values)%2 != 0 {
		return nil, fmt.Errorf("invalid dict call: number of arguments must be even")
	}
	m := make(map[string]interface{}, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict keys must be strings")
		}
		m[key] = values[i+1]
	}
	return m, nil
}

func mustLoadTemplates(tmplPath string) map[string]*template.Template {
	templates := make(map[string]*template.Template)
	files, err := os.ReadDir(tmplPath)
	if err != nil {
		log.Fatal(err)
	}

	for _, f := range files {
		if filepath.Ext(f.Name()) == ".html" && f.Name() != baseTemplate && f.Name() != "partials.html" {
			templates[f.Name()] = template.Must(template.New(baseTemplate).Funcs(
				template.FuncMap{"sub": sub, "add": add, "dict": dict},
			).ParseFiles(
				path.Join(tmplPath, baseTemplate),
				path.Join(tmplPath, f.Name()),
				path.Join(tmplPath, "partials.html"),
			),
			)
			// fmt.Printf("Template %s loaded successfully\n", f.Name())
		}
	}
	return templates
}

func startTemplateReloader(h *handler.Handler, tmplPath string) {
	if os.Getenv("ENV") == "development" {
		ticker := time.NewTicker(templateReloadInterval)
		go func() {
			for range ticker.C {
				h.Templates = mustLoadTemplates(tmplPath)
			}
		}()
	}
}

// fetchPublicConfig loads public config from backend API.
func fetchPublicConfig() config.Public {
	var pub config.Public
	resp, err := http.Get("http://api:8080/v1/public_config")
	if err != nil {
		log.Printf("failed to fetch public config: %v", err)
		return pub
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("fetch public config bad status: %d", resp.StatusCode)
		return pub
	}
	if err := json.NewDecoder(resp.Body).Decode(&pub); err != nil {
		log.Printf("failed to decode public config: %v", err)
		return pub
	}
	return pub
}
