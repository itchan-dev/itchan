package pg

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/itchan-dev/itchan/internal/config"
	"github.com/itchan-dev/itchan/internal/domain"

	_ "github.com/lib/pq"
)

type Storage struct {
	db *sql.DB
}

func New(cfg config.Pg) (*Storage, error) {
	log.Print("Connecting to db")
	db, err := Connect(cfg)
	if err != nil {
		return nil, err
	}

	log.Print("Initializing db")
	err = Init(db, cfg)
	if err != nil {
		return nil, err
	}

	return &Storage{db}, nil
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

func Init(db *sql.DB, cfg config.Pg) error {
	query, err := os.ReadFile(cfg.InitPath)
	if err != nil {
		return err
	}

	_, err = db.Exec(string(query))
	if err != nil {
		return err
	}

	return nil
}

func (s *Storage) Cleanup() error {
	return s.db.Close()
}

// Saves user to db
func (s *Storage) SaveUser(email string, passHash []byte) (int64, error) {
	var id int64
	err := s.db.QueryRow("INSERT INTO users(email, pass_hash) VALUES($1, $2) RETURNING id", email, string(passHash)).Scan(&id)
	if err != nil {
		return 0, err
	}

	return id, nil
}

// Get user from db
//
// If user doesn't exist, return error
func (s *Storage) User(email string) (*domain.User, error) {
	result := s.db.QueryRow("SELECT id, email, pass_hash FROM users WHERE email = $1", email)

	var user domain.User
	err := result.Scan(&user.Id, &user.Email, &user.PassHash)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// Delete user from db
//
// If user doesn't exist, return error
func (s *Storage) DeleteUser(email string) error {
	result, err := s.db.Exec("DELETE FROM users WHERE email = $1", email)
	if err != nil {
		return err
	}

	rowsDeleted, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsDeleted == 0 {
		return fmt.Errorf("no such user: %s", email)
	}

	return nil
}
