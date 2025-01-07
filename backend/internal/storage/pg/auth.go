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
func (s *Storage) SaveUser(user *domain.User) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return -1, err
	}
	defer tx.Rollback() // The rollback will be ignored if the tx has been committed later in the function.

	var id int64
	err = tx.QueryRow("INSERT INTO users(email, pass_hash, is_admin) VALUES($1, $2, $3) RETURNING id",
		user.Email, user.PassHash, user.Admin).Scan(&id)
	if err != nil {
		return -1, err
	}

	if err := tx.Commit(); err != nil {
		return -1, fmt.Errorf("failed to commit transaction: %w", err)
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

func (s *Storage) UpdatePassword(email, passHash string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // The rollback will be ignored if the tx has been committed later in the function.

	result, err := tx.Exec("UPDATE users SET pass_hash = $1 WHERE email = $2", passHash, email)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return &internal_errors.ErrorWithStatusCode{Message: "User not found", StatusCode: 404}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
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

func (s *Storage) SaveConfirmationData(data *domain.ConfirmationData) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // The rollback will be ignored if the tx has been committed later in the function.

	_, err = tx.Exec("INSERT INTO confirmation_data(email, new_pass_hash, confirmation_code_hash, confirmation_expires) VALUES($1, $2, $3, $4)",
		data.Email, data.NewPassHash, data.ConfirmationCodeHash, data.Expires)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func (s *Storage) ConfirmationData(email string) (*domain.ConfirmationData, error) {
	var data domain.ConfirmationData
	err := s.db.QueryRow("SELECT email, new_pass_hash, confirmation_code_hash, (confirmation_expires at time zone 'utc') FROM confirmation_data WHERE email = $1", email).Scan(
		&data.Email, &data.NewPassHash, &data.ConfirmationCodeHash, &data.Expires)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &internal_errors.ErrorWithStatusCode{Message: "ConfirmationData not found", StatusCode: 404}
		}
		return nil, err
	}
	return &data, nil
}

func (s *Storage) DeleteConfirmationData(email string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // The rollback will be ignored if the tx has been committed later in the function.

	result, err := tx.Exec("DELETE FROM confirmation_data WHERE email = $1", email)
	if err != nil {
		return err
	}
	rowsDeleted, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsDeleted == 0 {
		return &internal_errors.ErrorWithStatusCode{Message: "ConfirmationData not found", StatusCode: 404}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}
