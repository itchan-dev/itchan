package setup

import (
	"context"
	"fmt"
	"html/template"

	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/itchan-dev/itchan/frontend/internal/apiclient"
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
	startTemplateReloader(h, tmplPath)

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

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func bytesToMB(bytes int64) int64 {
	return bytes / (1024 * 1024)
}

func mimeTypeExtensions(mimeTypes []string) string {
	if len(mimeTypes) == 0 {
		return ""
	}
	var exts []string
	for _, mime := range mimeTypes {
		// Split on "/" and take the second part (e.g., "image/jpeg" -> "jpeg")
		parts := splitString(mime, "/")
		if len(parts) == 2 {
			exts = append(exts, parts[1])
		}
	}
	return joinStrings(exts, ", ")
}

func splitString(s, sep string) []string {
	if s == "" {
		return []string{}
	}
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])
	return result
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

func dict(values ...any) (map[string]interface{}, error) {
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

func derefStr(value *string) string {
	return *value
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
	var result []string
	result = append(result, images...)
	result = append(result, videos...)
	return joinStrings(result, ",")
}

var functionMap template.FuncMap = template.FuncMap{
	"sub":                   sub,
	"add":                   add,
	"dict":                  dict,
	"hasPrefix":             hasPrefix,
	"bytesToMB":             bytesToMB,
	"mimeTypeExtensions":    mimeTypeExtensions,
	"formatAcceptMimeTypes": formatAcceptMimeTypes,
	"derefStr":              derefStr,
	"thumbDims":             thumbDims,
	"join":                  joinStrings,
	"list": func() []any {
		return []any{}
	},
	"append": func(list []any, item any) []any {
		return append(list, item)
	},
}

func mustLoadTemplates(tmplPath string) map[string]*template.Template {
	templates := make(map[string]*template.Template)
	files, err := os.ReadDir(tmplPath)
	if err != nil {
		logger.Log.Error("fatal error", "error", err)
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
