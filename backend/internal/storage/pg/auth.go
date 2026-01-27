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

// User is a public, read-only method to fetch a user by their email hash. It uses
// the main database connection pool for efficiency.
func (s *Storage) User(emailHash []byte) (domain.User, error) {
	return s.user(s.db, emailHash)
}

// UpdatePassword is the public entry point for changing a user's password.
// It manages the transaction for this security-sensitive operation.
func (s *Storage) UpdatePassword(emailHash []byte, newPasswordHash domain.Password) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.withTx(ctx, func(tx *sql.Tx) error {
		return s.updatePassword(tx, emailHash, newPasswordHash)
	})
}

// DeleteUser is the public entry point for deleting a user account.
// It wraps the deletion in a transaction. The database schema's ON DELETE
// CASCADE constraints will handle cleaning up related data (e.g., confirmation data).
func (s *Storage) DeleteUser(emailHash []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.withTx(ctx, func(tx *sql.Tx) error {
		return s.deleteUser(tx, emailHash)
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
func (s *Storage) ConfirmationData(emailHash []byte) (domain.ConfirmationData, error) {
	return s.confirmationData(s.db, emailHash)
}

// DeleteConfirmationData is the public entry point for removing used or expired
// confirmation data.
func (s *Storage) DeleteConfirmationData(emailHash []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.withTx(ctx, func(tx *sql.Tx) error {
		return s.deleteConfirmationData(tx, emailHash)
	})
}

// =========================================================================
// Internal Methods (Core Database Logic)
// These methods accept a Querier and are transaction-agnostic.
// =========================================================================

// saveUser contains the core logic for inserting a new user record.
// It expects a domain.User with already-encrypted email fields.
func (s *Storage) saveUser(q Querier, user domain.User) (domain.UserId, error) {
	var id int64
	err := q.QueryRow(
		"INSERT INTO users(email_encrypted, email_domain, email_hash, password_hash, is_admin) VALUES($1, $2, $3, $4, $5) RETURNING id",
		user.EmailEncrypted, user.EmailDomain, user.EmailHash, user.PassHash, user.Admin,
	).Scan(&id)
	if err != nil {
		return -1, fmt.Errorf("failed to insert user: %w", err)
	}
	return id, nil
}

// user contains the core logic for fetching a single user record by email hash.
func (s *Storage) user(q Querier, emailHash []byte) (domain.User, error) {
	var user domain.User
	err := q.QueryRow(
		"SELECT id, email_encrypted, email_domain, email_hash, password_hash, is_admin, created_at FROM users WHERE email_hash = $1",
		emailHash,
	).Scan(&user.Id, &user.EmailEncrypted, &user.EmailDomain, &user.EmailHash, &user.PassHash, &user.Admin, &user.CreatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.User{}, &internal_errors.ErrorWithStatusCode{Message: "User not found", StatusCode: http.StatusNotFound}
		}
		return domain.User{}, fmt.Errorf("failed to query user: %w", err)
	}

	return user, nil
}

// updatePassword contains the core logic for updating a user's password hash.
func (s *Storage) updatePassword(q Querier, emailHash []byte, newPasswordHash domain.Password) error {
	result, err := q.Exec("UPDATE users SET password_hash = $1 WHERE email_hash = $2", newPasswordHash, emailHash)
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
func (s *Storage) deleteUser(q Querier, emailHash []byte) error {
	result, err := q.Exec("DELETE FROM users WHERE email_hash = $1", emailHash)
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
// It uses the email hash from the ConfirmationData struct.
func (s *Storage) saveConfirmationData(q Querier, data domain.ConfirmationData) error {
	_, err := q.Exec(`
        INSERT INTO confirmation_data(email_hash, password_hash, confirmation_code_hash, expires_at)
        VALUES($1, $2, $3, $4)`,
		data.EmailHash, data.PasswordHash, data.ConfirmationCodeHash, data.Expires,
	)
	if err != nil {
		return fmt.Errorf("failed to insert confirmation data: %w", err)
	}
	return nil
}

// confirmationData contains the core logic for fetching confirmation data.
func (s *Storage) confirmationData(q Querier, emailHash []byte) (domain.ConfirmationData, error) {
	var data domain.ConfirmationData
	err := q.QueryRow(`
        SELECT email_hash, password_hash, confirmation_code_hash, (expires_at at time zone 'utc')
        FROM confirmation_data WHERE email_hash = $1`,
		emailHash,
	).Scan(&data.EmailHash, &data.PasswordHash, &data.ConfirmationCodeHash, &data.Expires)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ConfirmationData{}, &internal_errors.ErrorWithStatusCode{Message: "Confirmation data not found", StatusCode: http.StatusNotFound}
		}
		return domain.ConfirmationData{}, fmt.Errorf("failed to query confirmation data: %w", err)
	}

	return data, nil
}

// deleteConfirmationData contains the core logic for deleting confirmation data.
func (s *Storage) deleteConfirmationData(q Querier, emailHash []byte) error {
	result, err := q.Exec("DELETE FROM confirmation_data WHERE email_hash = $1", emailHash)
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

// =========================================================================
// Invite Code Methods (for invite-based registration system)
// =========================================================================

// SaveInviteCode saves a new invite code to the database
func (s *Storage) SaveInviteCode(invite domain.InviteCode) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.withTx(ctx, func(tx *sql.Tx) error {
		return s.saveInviteCode(tx, invite)
	})
}

// InviteCodeByHash fetches an invite code by its hash
func (s *Storage) InviteCodeByHash(codeHash string) (domain.InviteCode, error) {
	return s.inviteCodeByHash(s.db, codeHash)
}

// GetInvitesByUser returns all invite codes created by a user
func (s *Storage) GetInvitesByUser(userId domain.UserId) ([]domain.InviteCode, error) {
	return s.getInvitesByUser(s.db, userId)
}

// CountActiveInvites returns the number of active (unused, unexpired) invites for a user
func (s *Storage) CountActiveInvites(userId domain.UserId) (int, error) {
	return s.countActiveInvites(s.db, userId)
}

// MarkInviteUsed marks an invite code as used by a specific user
func (s *Storage) MarkInviteUsed(codeHash string, usedBy domain.UserId) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.withTx(ctx, func(tx *sql.Tx) error {
		return s.markInviteUsed(tx, codeHash, usedBy)
	})
}

// DeleteInviteCode deletes an invite code by its hash
func (s *Storage) DeleteInviteCode(codeHash string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.withTx(ctx, func(tx *sql.Tx) error {
		return s.deleteInviteCode(tx, codeHash)
	})
}

// DeleteInvitesByUser deletes all unused invite codes created by a user
func (s *Storage) DeleteInvitesByUser(userId domain.UserId) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.withTx(ctx, func(tx *sql.Tx) error {
		return s.deleteInvitesByUser(tx, userId)
	})
}

// =========================================================================
// Internal Invite Methods (Core Database Logic)
// =========================================================================

func (s *Storage) saveInviteCode(q Querier, invite domain.InviteCode) error {
	_, err := q.Exec(`
		INSERT INTO invite_codes(code_hash, created_by, created_at, expires_at)
		VALUES($1, $2, $3, $4)`,
		invite.CodeHash, invite.CreatedBy, invite.CreatedAt, invite.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert invite code: %w", err)
	}
	return nil
}

func (s *Storage) inviteCodeByHash(q Querier, codeHash string) (domain.InviteCode, error) {
	row := q.QueryRow(`
		SELECT code_hash, created_by, created_at, expires_at, used_by, used_at
		FROM invite_codes
		WHERE code_hash = $1`,
		codeHash,
	)

	invite, err := scanInviteCode(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.InviteCode{}, &internal_errors.ErrorWithStatusCode{
				Message:    "Invite code not found",
				StatusCode: http.StatusNotFound,
			}
		}
		return domain.InviteCode{}, fmt.Errorf("failed to query invite code: %w", err)
	}

	return invite, nil
}

func (s *Storage) getInvitesByUser(q Querier, userId domain.UserId) ([]domain.InviteCode, error) {
	rows, err := q.Query(`
		SELECT code_hash, created_by, created_at, expires_at, used_by, used_at
		FROM invite_codes
		WHERE created_by = $1
		ORDER BY created_at DESC`,
		userId,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query user invites: %w", err)
	}
	defer rows.Close()

	var invites []domain.InviteCode
	for rows.Next() {
		invite, err := scanInviteCode(rows)
		if err != nil {
			return nil, err
		}
		invites = append(invites, invite)
	}

	return invites, rows.Err()
}

func (s *Storage) countActiveInvites(q Querier, userId domain.UserId) (int, error) {
	var count int

	err := q.QueryRow(`
		SELECT COUNT(*)
		FROM invite_codes
		WHERE created_by = $1
		  AND used_by IS NULL
		  AND expires_at > now()`,
		userId,
	).Scan(&count)

	if err != nil {
		return 0, fmt.Errorf("failed to count active invites: %w", err)
	}

	return count, nil
}

func (s *Storage) markInviteUsed(q Querier, codeHash string, usedBy domain.UserId) error {
	now := time.Now().UTC()

	result, err := q.Exec(`
		UPDATE invite_codes
		SET used_by = $1, used_at = $2
		WHERE code_hash = $3
		  AND used_by IS NULL`,
		usedBy, now, codeHash,
	)
	if err != nil {
		return fmt.Errorf("failed to mark invite as used: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rows == 0 {
		return &internal_errors.ErrorWithStatusCode{
			Message:    "Invite code already used or not found",
			StatusCode: http.StatusConflict,
		}
	}

	return nil
}

func (s *Storage) deleteInviteCode(q Querier, codeHash string) error {
	result, err := q.Exec(`
		DELETE FROM invite_codes
		WHERE code_hash = $1`,
		codeHash,
	)
	if err != nil {
		return fmt.Errorf("failed to delete invite code: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rows == 0 {
		return &internal_errors.ErrorWithStatusCode{
			Message:    "Invite code not found",
			StatusCode: http.StatusNotFound,
		}
	}

	return nil
}

func (s *Storage) deleteInvitesByUser(q Querier, userId domain.UserId) error {
	_, err := q.Exec(`
		DELETE FROM invite_codes
		WHERE created_by = $1
		  AND used_by IS NULL`,
		userId,
	)
	if err != nil {
		return fmt.Errorf("failed to delete user invites: %w", err)
	}

	return nil
}

// scanInviteCode is a helper function to scan invite codes from rows
func scanInviteCode(scanner interface {
	Scan(dest ...any) error
}) (domain.InviteCode, error) {
	var invite domain.InviteCode
	var usedBy sql.NullInt64
	var usedAt sql.NullTime

	err := scanner.Scan(
		&invite.CodeHash,
		&invite.CreatedBy,
		&invite.CreatedAt,
		&invite.ExpiresAt,
		&usedBy,
		&usedAt,
	)

	if err != nil {
		return domain.InviteCode{}, err
	}

	if usedBy.Valid {
		userId := domain.UserId(usedBy.Int64)
		invite.UsedBy = &userId
	}

	if usedAt.Valid {
		t := usedAt.Time.UTC()
		invite.UsedAt = &t
	}

	return invite, nil
}
