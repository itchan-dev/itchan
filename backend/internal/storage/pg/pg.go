package pg

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/itchan-dev/itchan/shared/config"

	_ "github.com/lib/pq"
)

type Storage struct {
	db  *sql.DB
	cfg *config.Config
}

func New(cfg *config.Config) (*Storage, error) {
	log.Print("Connecting to db")
	db, err := Connect(cfg)
	if err != nil {
		return nil, err
	}
	log.Print("Succesfully connected to db")
	return &Storage{db, cfg}, nil
}

func Connect(cfg *config.Config) (*sql.DB, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Private.Pg.Host, cfg.Private.Pg.Port, cfg.Private.Pg.User, cfg.Private.Pg.Password, cfg.Private.Pg.Dbname)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	return db, nil
}

func (s *Storage) Cleanup() error {
	return s.db.Close()
}

func getViewName(boardName string) string {
	return fmt.Sprintf("board_%s_view", boardName)
}
