package pg

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/itchan-dev/itchan/internal/config"

	_ "github.com/lib/pq"
)

type Storage struct {
	db  *sql.DB
	cfg *config.Pg
}

func New(cfg config.Pg) (*Storage, error) {
	log.Print("Connecting to db")
	db, err := Connect(cfg)
	if err != nil {
		return nil, err
	}

	return &Storage{db, &cfg}, nil
}

func Connect(cfg config.Pg) (*sql.DB, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Dbname)

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
