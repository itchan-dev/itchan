package setup

import (
	"context"
	"time"

	"github.com/itchan-dev/itchan/backend/internal/handler"
	"github.com/itchan-dev/itchan/backend/internal/service"
	"github.com/itchan-dev/itchan/backend/internal/storage/fs"
	"github.com/itchan-dev/itchan/backend/internal/storage/pg"
	"github.com/itchan-dev/itchan/backend/internal/utils"
	"github.com/itchan-dev/itchan/backend/internal/utils/email"
	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/jwt"
	"github.com/itchan-dev/itchan/shared/middleware/board_access"
)

// Dependencies struct to hold all initialized dependencies.
type Dependencies struct {
	Storage      *pg.Storage
	MediaStorage *fs.Storage
	Handler      *handler.Handler
	AccessData   *board_access.BoardAccess
	Jwt          jwt.JwtService
	CancelFunc   context.CancelFunc
}

// SetupDependencies initializes all dependencies required for the application.
func SetupDependencies(cfg *config.Config) (*Dependencies, error) {
	ctx, cancel := context.WithCancel(context.Background())
	storage, err := pg.New(ctx, cfg)
	if err != nil {
		cancel()
		return nil, err
	}

	// Initialize filesystem storage for media files
	mediaStorage, err := fs.New("./media")
	if err != nil {
		cancel()
		return nil, err
	}

	accessData := board_access.New()
	accessData.StartBackgroundUpdate(1*time.Minute, storage)

	// Initialize garbage collector for orphaned media files
	// Safety threshold: 24 hours - files must be at least 24h old before deletion
	// Cleanup interval: runs daily at roughly the same time
	mediaGC := service.NewMediaGarbageCollector(storage, mediaStorage, 24*time.Hour)
	mediaGC.StartBackgroundCleanup(ctx, 24*time.Hour)

	email := email.New(&cfg.Private.Email)
	jwt := jwt.New(cfg.JwtKey(), cfg.JwtTTL())

	auth := service.NewAuth(storage, email, jwt, &cfg.Public)
	board := service.NewBoard(storage, utils.New(&cfg.Public), mediaStorage)
	message := service.NewMessage(storage, &utils.MessageValidator{Сfg: &cfg.Public}, mediaStorage, &cfg.Public)
	thread := service.NewThread(storage, &utils.ThreadTitleValidator{Сfg: &cfg.Public}, message, mediaStorage)

	// Initialize garbage collector for old threads
	// Cleanup interval: runs every 5 minutes to keep boards at MaxThreadCount
	threadGC := service.NewThreadGarbageCollector(storage, thread, cfg.Public.MaxThreadCount)
	threadGC.StartBackgroundCleanup(ctx, 5*time.Minute)

	h := handler.New(auth, board, thread, message, mediaStorage, cfg)

	return &Dependencies{
		Storage:      storage,
		MediaStorage: mediaStorage,
		Handler:      h,
		AccessData:   accessData,
		Jwt:          jwt,
		CancelFunc:   cancel,
	}, nil
}
