package setup

import (
	"fmt"
	"html/template"
	"log"
	"os"
	"path"
	"time"

	"github.com/itchan-dev/itchan/frontend/internal/handler"
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
}

func SetupDependencies() *Dependencies {
	templates := mustLoadTemplates(tmplPath)
	h := handler.New(templates)
	startTemplateReloader(h, tmplPath)
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}
	jwtSvc := jwt.New(jwtSecret, 2629800000000000) // 1 month expiration

	return &Dependencies{Handler: h, Jwt: jwtSvc}
}

func sub(a, b int) int { return a - b }
func add(a, b int) int { return a + b }

func mustLoadTemplates(tmplPath string) map[string]*template.Template {
	templates := make(map[string]*template.Template)
	files, err := os.ReadDir(tmplPath)
	if err != nil {
		log.Fatal(err)
	}

	for _, f := range files {
		if f.Name() != baseTemplate {
			templates[f.Name()] = template.Must(template.New(baseTemplate).Funcs(
				template.FuncMap{"sub": sub, "add": add},
			).ParseFiles(
				path.Join(tmplPath, baseTemplate),
				path.Join(tmplPath, f.Name()),
			),
			)
			fmt.Printf("Template %s loaded successfully", f.Name())
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
