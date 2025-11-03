package storage

import (
	"database/sql"
	"fmt"

	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/storage/pg"
	_ "github.com/lib/pq" // PostgreSQL driver
)

// Storage provides database connection and common queries.
// This is a lightweight shared storage implementation for services
// that need basic database access without the full backend storage layer.
// It wraps the shared pg storage primitives with application-specific queries.
type Storage struct {
	db *sql.DB
}

// New creates a new storage instance with database connection.
// Uses lightweight connection pool settings suitable for frontend/worker services.
func New(cfg *config.Config) (*Storage, error) {
	db, err := pg.Connect(cfg, pg.LightweightConnectionConfig())
	if err != nil {
		return nil, err
	}
	return &Storage{db: db}, nil
}

// GetBoardsWithPermissions returns a map of board short names to their allowed email domains.
// Returns nil for boards without restrictions (public boards).
// This method is used by the board_access middleware to enforce access control.
func (s *Storage) GetBoardsWithPermissions() (map[string][]string, error) {
	rows, err := s.db.Query(`
		SELECT board_short_name, allowed_email_domain
		FROM board_permissions
		ORDER BY board_short_name, allowed_email_domain
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query board permissions: %w", err)
	}
	defer rows.Close()

	permissions := make(map[string][]string)
	for rows.Next() {
		var boardShortName string
		var allowedDomain string
		if err := rows.Scan(&boardShortName, &allowedDomain); err != nil {
			return nil, fmt.Errorf("failed to scan board permission row: %w", err)
		}
		permissions[boardShortName] = append(permissions[boardShortName], allowedDomain)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return permissions, nil
}

// Cleanup closes the database connection pool.
func (s *Storage) Cleanup() {
	if s.db != nil {
		s.db.Close()
	}
}
