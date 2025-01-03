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
	cfg *config.Public
}

func New(cfg config.Public) (*Storage, error) {
	log.Print("Connecting to db")
	db, err := Connect(cfg)
	if err != nil {
		return nil, err
	}
	log.Print("Succesfully connected to db")
	return &Storage{db, &cfg}, nil
}

func Connect(cfg config.Public) (*sql.DB, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Pg.Host, cfg.Pg.Port, cfg.Pg.User, cfg.Pg.Password, cfg.Pg.Dbname)

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
