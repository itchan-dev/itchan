package setup

import (
	"context"
	"time"

	"github.com/itchan-dev/itchan/backend/internal/handler"
	"github.com/itchan-dev/itchan/backend/internal/middleware/board_access"
	"github.com/itchan-dev/itchan/backend/internal/service"
	"github.com/itchan-dev/itchan/backend/internal/storage/pg"
	"github.com/itchan-dev/itchan/backend/internal/utils"
	"github.com/itchan-dev/itchan/backend/internal/utils/email"
	"github.com/itchan-dev/itchan/backend/internal/utils/jwt"
	"github.com/itchan-dev/itchan/shared/config"
)

// Dependencies struct to hold all initialized dependencies.
type Dependencies struct {
	Storage    *pg.Storage
	Handler    *handler.Handler
	AccessData *board_access.BoardAccess
	Jwt        jwt.JwtService
	CancelFunc context.CancelFunc
}

// SetupDependencies initializes all dependencies required for the application.
func SetupDependencies(cfg *config.Config) (*Dependencies, error) {
	ctx, cancel := context.WithCancel(context.Background())
	storage, err := pg.New(ctx, cfg)
	if err != nil {
		cancel()
		return nil, err
	}

	accessData := board_access.New()
	accessData.StartBackgroundUpdate(1*time.Minute, storage)

	email := email.New(&cfg.Private.Email)
	jwt := jwt.New(cfg.JwtKey(), cfg.JwtTTL())

	auth := service.NewAuth(storage, email, jwt)
	board := service.NewBoard(storage, &utils.BoardNameValidator{})
	thread := service.NewThread(storage, &utils.ThreadTitleValidator{})
	message := service.NewMessage(storage, &utils.MessageValidator{})

	h := handler.New(auth, board, thread, message, cfg)

	return &Dependencies{
		Storage:    storage,
		Handler:    h,
		AccessData: accessData,
		Jwt:        jwt,
		CancelFunc: cancel,
	}, nil
}
