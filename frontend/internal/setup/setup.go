package setup

import (
	"context"
	"fmt"
	"html/template"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/itchan-dev/itchan/frontend/internal/apiclient"
	frontend_domain "github.com/itchan-dev/itchan/frontend/internal/domain"
	"github.com/itchan-dev/itchan/frontend/internal/handler"
	"github.com/itchan-dev/itchan/frontend/internal/markdown"
	"github.com/itchan-dev/itchan/shared/blacklist"
	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/jwt"
	"github.com/itchan-dev/itchan/shared/logger"
	"github.com/itchan-dev/itchan/shared/middleware"
	"github.com/itchan-dev/itchan/shared/middleware/board_access"
	"github.com/itchan-dev/itchan/shared/storage"
)

const (
	baseTemplate           = "base.html"
	tmplPath               = "./templates"
	templateReloadInterval = 5 * time.Second
)

type Dependencies struct {
	Handler        *handler.Handler
	Jwt            jwt.JwtService
	Public         config.Public
	Storage        *storage.Storage
	AccessData     *board_access.BoardAccess
	BlacklistCache *blacklist.Cache
	AuthMiddleware *middleware.Auth
	CancelFunc     context.CancelFunc
}

func SetupDependencies(cfg *config.Config) (*Dependencies, error) {
	// Create cancellable context for background tasks
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize database connection
	store, err := storage.New(cfg)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Initialize board access data with background updates
	accessData := board_access.New()
	accessData.StartBackgroundUpdate(ctx, 1*time.Minute, store)

	// Load templates and other dependencies
	templates := mustLoadTemplates(tmplPath)
	textProcessor := markdown.New(&cfg.Public)
	apiClient := apiclient.New("http://api:8080")

	// Get media path from environment or use default
	mediaPath := os.Getenv("MEDIA_PATH")
	if mediaPath == "" {
		mediaPath = "./media" // Relative to working directory
	}

	h := handler.New(templates, cfg.Public, textProcessor, apiClient, mediaPath)
	startTemplateReloader(ctx, h, tmplPath)

	jwtService := jwt.New(cfg.JwtKey(), cfg.JwtTTL())

	// Initialize blacklist cache
	blacklistCache := blacklist.NewCache(store, cfg.JwtTTL())

	// Load initial cache synchronously
	logger.Log.Info("Initializing blacklist cache...")
	if err := blacklistCache.Update(); err != nil {
		cancel()
		store.Cleanup()
		return nil, fmt.Errorf("failed to initialize blacklist cache: %w", err)
	}

	// Start background updates
	interval := time.Duration(cfg.Public.BlacklistCacheInterval) * time.Second
	blacklistCache.StartBackgroundUpdate(ctx, interval)

	// Create auth middleware
	secureCookies := cfg.Public.SecureCookies
	authMiddleware := middleware.NewAuth(jwtService, blacklistCache, secureCookies)

	return &Dependencies{
		Handler:        h,
		Jwt:            jwtService,
		Public:         cfg.Public,
		Storage:        store,
		AccessData:     accessData,
		BlacklistCache: blacklistCache,
		AuthMiddleware: authMiddleware,
		CancelFunc:     cancel,
	}, nil
}

func sub(a, b int) int { return a - b }
func add(a, b int) int { return a + b }

func bytesToMB(bytes int64) int64 {
	return bytes / (1024 * 1024)
}

func mimeTypeExtensions(mimeTypes []string) string {
	var exts []string
	for _, mime := range mimeTypes {
		if parts := strings.SplitN(mime, "/", 2); len(parts) == 2 {
			exts = append(exts, parts[1])
		}
	}
	return strings.Join(exts, ", ")
}

func dict(values ...any) (map[string]any, error) {
	if len(values)%2 != 0 {
		return nil, fmt.Errorf("invalid dict call: number of arguments must be even")
	}
	m := make(map[string]any, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict keys must be strings")
		}
		m[key] = values[i+1]
	}
	return m, nil
}

// thumbDims computes display dimensions for a thumbnail, preserving aspect ratio.
// Returns {"W": width, "H": height} scaled to fit within maxSize.
func thumbDims(imgWidth, imgHeight *int, maxSize int) map[string]int {
	if imgWidth == nil || imgHeight == nil || *imgWidth == 0 || *imgHeight == 0 {
		return map[string]int{"W": 0, "H": 0}
	}
	w, h := *imgWidth, *imgHeight
	if w <= maxSize && h <= maxSize {
		return map[string]int{"W": w, "H": h}
	}
	if w > h {
		return map[string]int{"W": maxSize, "H": h * maxSize / w}
	}
	return map[string]int{"W": w * maxSize / h, "H": maxSize}
}

func formatAcceptMimeTypes(images, videos []string) string {
	return strings.Join(append(images, videos...), ",")
}

var functionMap template.FuncMap = template.FuncMap{
	"sub":                   sub,
	"add":                   add,
	"dict":                  dict,
	"postData": func(msg *frontend_domain.Message, common frontend_domain.CommonTemplateData) *frontend_domain.PostData {
		return &frontend_domain.PostData{Message: msg, Common: &common}
	},
	"bytesToMB":             bytesToMB,
	"mimeTypeExtensions":    mimeTypeExtensions,
	"formatAcceptMimeTypes": formatAcceptMimeTypes,
	"thumbDims":             thumbDims,
	"join":                  strings.Join,
}

func mustLoadTemplates(tmplPath string) map[string]*template.Template {
	templates := make(map[string]*template.Template)
	files, err := os.ReadDir(tmplPath)
	if err != nil {
		panic(fmt.Errorf("mustLoadTemplates: %w", err))
	}

	// Create a standalone partials template for API endpoints
	templates["partials"] = template.Must(
		template.New("partials.html").
			Funcs(functionMap).
			ParseFiles(
				path.Join(tmplPath, "partials.html"),
			),
	)

	for _, f := range files {
		if filepath.Ext(f.Name()) == ".html" && f.Name() != baseTemplate && f.Name() != "partials.html" {
			templates[f.Name()] = template.Must(
				template.New(baseTemplate).
					Funcs(functionMap).
					ParseFiles(
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

func startTemplateReloader(ctx context.Context, h *handler.Handler, tmplPath string) {
	if os.Getenv("ENV") == "development" {
		ticker := time.NewTicker(templateReloadInterval)
		go func() {
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					h.UpdateTemplates(mustLoadTemplates(tmplPath))
				case <-ctx.Done():
					return
				}
			}
		}()
	}
}
