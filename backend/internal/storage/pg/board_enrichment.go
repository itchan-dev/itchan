package pg

import (
	"fmt"

	"github.com/itchan-dev/itchan/shared/domain"
)

// enrichBoardsWithPermissions fetches and attaches allowed email domains to boards.
// It queries board_permissions table and populates the AllowedEmailDomains field
// of each board in the provided slice.
//
// Boards without permissions (public boards) will have nil AllowedEmailDomains.
// Boards with permissions (corporate boards) will have a non-empty slice.
func enrichBoardsWithPermissions(q Querier, boards []domain.Board) error {
	if len(boards) == 0 {
		return nil // No boards to enrich
	}

	// Load all board permissions in a single query
	permissions, err := getBoardsWithPermissions(q)
	if err != nil {
		return fmt.Errorf("failed to load board permissions: %w", err)
	}

	// Populate AllowedEmailDomains for each board
	for i := range boards {
		boardKey := string(boards[i].ShortName)
		if domains, exists := permissions[boardKey]; exists {
			boards[i].AllowedEmailDomains = domains
		}
		// If not exists, AllowedEmailDomains remains nil (public board)
	}

	return nil
}

// getBoardsWithPermissions queries all board permissions and returns a map
// of board short names to their allowed email domains.
func getBoardsWithPermissions(q Querier) (map[string][]string, error) {
	rows, err := q.Query(`
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
		return nil, fmt.Errorf("error iterating board permission rows: %w", err)
	}

	return permissions, nil
}
