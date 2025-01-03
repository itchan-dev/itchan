package pg

import (
	"database/sql"
	"errors"
	"fmt"

	internal_errors "github.com/itchan-dev/itchan/backend/internal/errors"
	"github.com/itchan-dev/itchan/shared/domain"
	_ "github.com/lib/pq"
)

// Saves user to db
func (s *Storage) SaveUser(email string, passHash []byte) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() // The rollback will be ignored if the tx has been committed later in the function.

	var id int64
	err = tx.QueryRow("INSERT INTO users(email, pass_hash) VALUES($1, $2) RETURNING id", email, string(passHash)).Scan(&id)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return id, nil
}

// Get user from db
//
// If user doesn't exist, return error
func (s *Storage) User(email string) (*domain.User, error) {
	var user domain.User
	err := s.db.QueryRow("SELECT id, email, pass_hash, is_admin FROM users WHERE email = $1", email).Scan(&user.Id, &user.Email, &user.PassHash, &user.Admin)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &internal_errors.ErrorWithStatusCode{Message: "User not found", StatusCode: 404}
		}
		return nil, err
	}
	return &user, nil
}

// Delete user from db
//
// If user doesn't exist, return error
func (s *Storage) DeleteUser(email string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // The rollback will be ignored if the tx has been committed later in the function.

	result, err := tx.Exec("DELETE FROM users WHERE email = $1", email)
	if err != nil {
		return err
	}
	rowsDeleted, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsDeleted == 0 {
		return &internal_errors.ErrorWithStatusCode{Message: "User not found", StatusCode: 404}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
