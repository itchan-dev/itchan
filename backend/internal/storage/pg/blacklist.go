package pg

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
	internal_errors "github.com/itchan-dev/itchan/shared/errors"
)

// =========================================================================
// Public Methods (satisfy the service.BlacklistStorage interface)
// =========================================================================

// GetRecentlyBlacklistedUsers fetches all user IDs that were blacklisted
// after the specified time. This is used for cache updates with TTL-based filtering.
func (s *Storage) GetRecentlyBlacklistedUsers(since time.Time) ([]domain.UserId, error) {
	return s.getRecentlyBlacklistedUsers(s.db, since)
}

// BlacklistUser adds a user to the blacklist. This is the public entry point
// that wraps the operation in a transaction.
func (s *Storage) BlacklistUser(userId domain.UserId, reason string, blacklistedBy domain.UserId) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.withTx(ctx, func(tx *sql.Tx) error {
		return s.blacklistUser(tx, userId, reason, blacklistedBy)
	})
}

// UnblacklistUser removes a user from the blacklist. This is the public entry point
// that wraps the operation in a transaction.
func (s *Storage) UnblacklistUser(userId domain.UserId) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.withTx(ctx, func(tx *sql.Tx) error {
		return s.unblacklistUser(tx, userId)
	})
}

// IsUserBlacklisted checks if a specific user is currently blacklisted.
// This is a read-only operation used for direct DB checks (e.g., at login).
func (s *Storage) IsUserBlacklisted(userId domain.UserId) (bool, error) {
	return s.isUserBlacklisted(s.db, userId)
}

// GetBlacklistedUsersWithDetails retrieves all blacklisted users with their full details
// (email, reason, blacklisted_at, blacklisted_by) for admin display purposes.
func (s *Storage) GetBlacklistedUsersWithDetails() ([]domain.BlacklistEntry, error) {
	return s.getBlacklistedUsersWithDetails(s.db)
}

// =========================================================================
// Internal Methods (Core Database Logic)
// These methods accept a Querier and are transaction-agnostic.
// =========================================================================

// getRecentlyBlacklistedUsers contains the core logic for fetching recently blacklisted users.
func (s *Storage) getRecentlyBlacklistedUsers(q Querier, since time.Time) ([]domain.UserId, error) {
	rows, err := q.Query(`
		SELECT user_id
		FROM user_blacklist
		WHERE blacklisted_at >= $1
		ORDER BY blacklisted_at DESC`,
		since,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query recently blacklisted users: %w", err)
	}
	defer rows.Close()

	var userIds []domain.UserId
	for rows.Next() {
		var userId domain.UserId
		if err := rows.Scan(&userId); err != nil {
			return nil, fmt.Errorf("failed to scan blacklisted user ID: %w", err)
		}
		userIds = append(userIds, userId)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating blacklisted users: %w", err)
	}

	return userIds, nil
}

// blacklistUser contains the core logic for inserting a blacklist entry.
func (s *Storage) blacklistUser(q Querier, userId domain.UserId, reason string, blacklistedBy domain.UserId) error {
	// Check if user is trying to blacklist themselves
	if userId == blacklistedBy {
		return &internal_errors.ErrorWithStatusCode{
			Message:    "Cannot blacklist yourself",
			StatusCode: http.StatusBadRequest,
		}
	}

	// Use INSERT ... ON CONFLICT to make operation idempotent
	// If user is already blacklisted, update the reason and timestamp
	_, err := q.Exec(`
		INSERT INTO user_blacklist (user_id, reason, blacklisted_by, blacklisted_at)
		VALUES ($1, $2, $3, NOW() AT TIME ZONE 'utc')
		ON CONFLICT (user_id)
		DO UPDATE SET
			reason = EXCLUDED.reason,
			blacklisted_by = EXCLUDED.blacklisted_by,
			blacklisted_at = NOW() AT TIME ZONE 'utc'`,
		userId, reason, blacklistedBy,
	)
	if err != nil {
		return fmt.Errorf("failed to blacklist user: %w", err)
	}

	return nil
}

// unblacklistUser contains the core logic for removing a blacklist entry.
func (s *Storage) unblacklistUser(q Querier, userId domain.UserId) error {
	result, err := q.Exec("DELETE FROM user_blacklist WHERE user_id = $1", userId)
	if err != nil {
		return fmt.Errorf("failed to unblacklist user: %w", err)
	}

	rowsDeleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows for unblacklist: %w", err)
	}

	if rowsDeleted == 0 {
		return &internal_errors.ErrorWithStatusCode{
			Message:    "User is not blacklisted",
			StatusCode: http.StatusNotFound,
		}
	}

	return nil
}

// isUserBlacklisted contains the core logic for checking blacklist status.
func (s *Storage) isUserBlacklisted(q Querier, userId domain.UserId) (bool, error) {
	var exists bool
	err := q.QueryRow("SELECT EXISTS(SELECT 1 FROM user_blacklist WHERE user_id = $1)", userId).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check blacklist status: %w", err)
	}
	return exists, nil
}

// getBlacklistedUsersWithDetails contains the core logic for fetching all blacklist entries with details.
func (s *Storage) getBlacklistedUsersWithDetails(q Querier) ([]domain.BlacklistEntry, error) {
	rows, err := q.Query(`
		SELECT
			ub.user_id,
			u.email,
			ub.blacklisted_at AT TIME ZONE 'utc' as blacklisted_at,
			ub.reason,
			ub.blacklisted_by
		FROM user_blacklist ub
		JOIN users u ON u.id = ub.user_id
		ORDER BY ub.blacklisted_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query blacklisted users with details: %w", err)
	}
	defer rows.Close()

	var entries []domain.BlacklistEntry
	for rows.Next() {
		var entry domain.BlacklistEntry
		if err := rows.Scan(
			&entry.UserId,
			&entry.Email,
			&entry.BlacklistedAt,
			&entry.Reason,
			&entry.BlacklistedBy,
		); err != nil {
			return nil, fmt.Errorf("failed to scan blacklist entry: %w", err)
		}
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating blacklist entries: %w", err)
	}

	return entries, nil
}
