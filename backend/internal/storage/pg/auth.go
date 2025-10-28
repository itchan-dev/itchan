package pg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
	internal_errors "github.com/itchan-dev/itchan/shared/errors"
	_ "github.com/lib/pq"
)

// =========================================================================
// Public Methods (satisfy the service.AuthStorage interface)
// =========================================================================

// SaveUser is the public entry point for creating a new user. It wraps the
// core logic in a transaction to ensure the operation is atomic.
func (s *Storage) SaveUser(user domain.User) (domain.UserId, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var id domain.UserId
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		var err error
		id, err = s.saveUser(tx, user)
		return err
	})
	return id, err
}

// User is a public, read-only method to fetch a user by their email. It uses
// the main database connection pool for efficiency.
func (s *Storage) User(email domain.Email) (domain.User, error) {
	return s.user(s.db, email)
}

// UpdatePassword is the public entry point for changing a user's password.
// It manages the transaction for this security-sensitive operation.
func (s *Storage) UpdatePassword(creds domain.Credentials) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.withTx(ctx, func(tx *sql.Tx) error {
		return s.updatePassword(tx, creds)
	})
}

// DeleteUser is the public entry point for deleting a user account.
// It wraps the deletion in a transaction. The database schema's ON DELETE
// CASCADE constraints will handle cleaning up related data (e.g., confirmation data).
func (s *Storage) DeleteUser(email domain.Email) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.withTx(ctx, func(tx *sql.Tx) error {
		return s.deleteUser(tx, email)
	})
}

// SaveConfirmationData is the public entry point for storing password reset
// or account confirmation tokens.
func (s *Storage) SaveConfirmationData(data domain.ConfirmationData) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.withTx(ctx, func(tx *sql.Tx) error {
		return s.saveConfirmationData(tx, data)
	})
}

// ConfirmationData is a public, read-only method to retrieve confirmation data.
func (s *Storage) ConfirmationData(email domain.Email) (domain.ConfirmationData, error) {
	return s.confirmationData(s.db, email)
}

// DeleteConfirmationData is the public entry point for removing used or expired
// confirmation data.
func (s *Storage) DeleteConfirmationData(email domain.Email) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.withTx(ctx, func(tx *sql.Tx) error {
		return s.deleteConfirmationData(tx, email)
	})
}

// =========================================================================
// Internal Methods (Core Database Logic)
// These methods accept a Querier and are transaction-agnostic.
// =========================================================================

// saveUser contains the core logic for inserting a new user record.
func (s *Storage) saveUser(q Querier, user domain.User) (domain.UserId, error) {
	var id int64
	err := q.QueryRow("INSERT INTO users(email, password_hash, is_admin) VALUES($1, $2, $3) RETURNING id",
		user.Email, user.PassHash, user.Admin).Scan(&id)
	if err != nil {
		return -1, fmt.Errorf("failed to insert user: %w", err)
	}
	return id, nil
}

// user contains the core logic for fetching a single user record by email.
func (s *Storage) user(q Querier, email domain.Email) (domain.User, error) {
	var user domain.User
	err := q.QueryRow("SELECT id, email, password_hash, is_admin FROM users WHERE email = $1", email).Scan(&user.Id, &user.Email, &user.PassHash, &user.Admin)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.User{}, &internal_errors.ErrorWithStatusCode{Message: "User not found", StatusCode: http.StatusNotFound}
		}
		return domain.User{}, fmt.Errorf("failed to query user: %w", err)
	}
	return user, nil
}

// updatePassword contains the core logic for updating a user's password hash.
func (s *Storage) updatePassword(q Querier, creds domain.Credentials) error {
	result, err := q.Exec("UPDATE users SET password_hash = $1 WHERE email = $2", creds.Password, creds.Email)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows for password update: %w", err)
	}
	if rowsAffected == 0 {
		return &internal_errors.ErrorWithStatusCode{Message: "User not found for password update", StatusCode: http.StatusNotFound}
	}
	return nil
}

// deleteUser contains the core logic for deleting a user record.
func (s *Storage) deleteUser(q Querier, email domain.Email) error {
	result, err := q.Exec("DELETE FROM users WHERE email = $1", email)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	rowsDeleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows for user deletion: %w", err)
	}
	if rowsDeleted == 0 {
		return &internal_errors.ErrorWithStatusCode{Message: "User not found for deletion", StatusCode: http.StatusNotFound}
	}
	return nil
}

// saveConfirmationData contains the core logic for inserting confirmation data.
func (s *Storage) saveConfirmationData(q Querier, data domain.ConfirmationData) error {
	_, err := q.Exec(`
        INSERT INTO confirmation_data(email, password_hash, confirmation_code_hash, expires_at)
        VALUES($1, $2, $3, $4)`,
		data.Email, data.PasswordHash, data.ConfirmationCodeHash, data.Expires,
	)
	if err != nil {
		return fmt.Errorf("failed to insert confirmation data: %w", err)
	}
	return nil
}

// confirmationData contains the core logic for fetching confirmation data.
func (s *Storage) confirmationData(q Querier, email domain.Email) (domain.ConfirmationData, error) {
	var data domain.ConfirmationData
	err := q.QueryRow(`
        SELECT email, password_hash, confirmation_code_hash, (expires_at at time zone 'utc')
        FROM confirmation_data WHERE email = $1`,
		email,
	).Scan(&data.Email, &data.PasswordHash, &data.ConfirmationCodeHash, &data.Expires)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ConfirmationData{}, &internal_errors.ErrorWithStatusCode{Message: "Confirmation data not found", StatusCode: http.StatusNotFound}
		}
		return domain.ConfirmationData{}, fmt.Errorf("failed to query confirmation data: %w", err)
	}
	return data, nil
}

// deleteConfirmationData contains the core logic for deleting confirmation data.
func (s *Storage) deleteConfirmationData(q Querier, email domain.Email) error {
	result, err := q.Exec("DELETE FROM confirmation_data WHERE email = $1", email)
	if err != nil {
		return fmt.Errorf("failed to delete confirmation data: %w", err)
	}
	rowsDeleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows for confirmation data deletion: %w", err)
	}
	if rowsDeleted == 0 {
		return &internal_errors.ErrorWithStatusCode{Message: "Confirmation data not found for deletion", StatusCode: http.StatusNotFound}
	}
	return nil
}
