package pg

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
	internal_errors "github.com/itchan-dev/itchan/shared/errors"
	"github.com/itchan-dev/itchan/shared/logger"

	"github.com/lib/pq"
)

var emptyAllowedEmailsError = errors.New("allowedEmails should be either nil or not empty")

//go:embed templates/board_view_template.sql
var viewTmpl string

//go:embed templates/partition_template.sql
var partitionTmpl string

//go:embed templates/partition_template_simple.sql
var partitionTmplSimple string

// =========================================================================
// Public Methods (satisfy the service.BoardStorage interface)
// =========================================================================

// CreateBoard is the public entry point for creating a new board.
// It wraps the entire board creation process, including metadata insertion,
// partition creation, and view creation, within a single atomic transaction.
// This guarantees that a board is either fully created or not created at all.
func (s *Storage) CreateBoard(creationData domain.BoardCreationData) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return s.withTx(ctx, func(tx *sql.Tx) error {
		return s.createBoard(tx, creationData)
	})
}

// DeleteBoard is the public entry point for deleting a board.
// It manages the transaction for this destructive operation, ensuring that the
// board's materialized view, all its table partitions, and its metadata
// are removed atomically.
func (s *Storage) DeleteBoard(shortName domain.BoardShortName) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return s.withTx(ctx, func(tx *sql.Tx) error {
		return s.deleteBoard(tx, shortName)
	})
}

// GetBoard is a public, read-only method for fetching a single page of a board's
// content. It delegates directly to the internal method using the main
// database connection pool.
func (s *Storage) GetBoard(shortName domain.BoardShortName, page int) (domain.Board, error) {
	return s.getBoard(s.db, shortName, page)
}

// GetBoards is a public, read-only method to fetch metadata for all boards.
func (s *Storage) GetBoards() ([]domain.Board, error) {
	return s.getBoards(s.db)
}

// GetBoardsByUser is a public, read-only method to fetch metadata for all
// boards a specific user is permitted to see.
func (s *Storage) GetBoardsByUser(user domain.User) ([]domain.Board, error) {
	return s.getBoardsByUser(s.db, user)
}

// GetActiveBoards is a public, read-only method used by the view refresh
// background process to find boards with recent activity.
func (s *Storage) GetActiveBoards(interval time.Duration) ([]domain.Board, error) {
	return s.getActiveBoards(s.db, interval)
}

// ThreadCount is a public, read-only utility method to count threads on a board.
func (s *Storage) ThreadCount(board domain.BoardShortName) (int, error) {
	return s.threadCount(s.db, board)
}

// LastThreadId is a public, read-only utility method to find the least recently
// bumped non-pinned thread, often used for pruning.
func (s *Storage) LastThreadId(board domain.BoardShortName) (domain.MsgId, error) {
	return s.lastThreadId(s.db, board)
}

// GetBoardsWithPermissions returns a map of board short names to their allowed email domains.
// Returns nil for boards without restrictions (public boards).
func (s *Storage) GetBoardsWithPermissions() (map[string][]string, error) {
	return getBoardsWithPermissions(s.db)
}

// =========================================================================
// Internal Methods (Core Database Logic)
// These methods accept a Querier and are transaction-agnostic.
// =========================================================================

// createBoard contains the core DDL and DML logic for creating all database
// objects associated with a new board. It must be executed within a transaction.
func (s *Storage) createBoard(q Querier, creationData domain.BoardCreationData) error {
	if creationData.AllowedEmails != nil && len(*creationData.AllowedEmails) == 0 {
		return fmt.Errorf("%w: allowed_emails cannot be empty", emptyAllowedEmailsError)
	}

	// Insert board metadata.
	_, err := q.Exec(`
        INSERT INTO boards (name, short_name) VALUES ($1, $2)`,
		creationData.Name, creationData.ShortName,
	)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" { // Unique violation
			return &internal_errors.ErrorWithStatusCode{
				Message: fmt.Sprintf("Board with short name '%s' already exists", creationData.ShortName), StatusCode: http.StatusConflict,
			}
		}
		return fmt.Errorf("failed to insert board metadata: %w", err)
	}

	// Insert board permissions if provided.
	if creationData.AllowedEmails != nil {
		for _, email := range *creationData.AllowedEmails {
			_, err = q.Exec(`
				INSERT INTO board_permissions (board_short_name, allowed_email_domain) VALUES ($1, $2)`,
				creationData.ShortName, email,
			)
			if err != nil {
				return fmt.Errorf("failed to insert board permission for '%s': %w", email, err)
			}
		}
	}

	// Create partition for threads table (requires sequence for auto-increment ID).
	threadsSeqName := fmt.Sprintf("threads_id_seq_%s", creationData.ShortName)
	threadsQuery := fmt.Sprintf(partitionTmpl,
		pq.QuoteIdentifier(threadsSeqName),
		PartitionName(creationData.ShortName, "threads"),
		pq.QuoteIdentifier("threads"),
		pq.QuoteLiteral(threadsSeqName),
		pq.QuoteLiteral(creationData.ShortName),
	)
	if _, err = q.Exec(threadsQuery); err != nil {
		return fmt.Errorf("failed to create threads partition for board '%s': %w", creationData.ShortName, err)
	}

	// Create partitions for tables without sequences (id is set explicitly).
	for _, table := range []string{"messages", "attachments", "message_replies"} {
		query := fmt.Sprintf(partitionTmplSimple,
			PartitionName(creationData.ShortName, table),
			pq.QuoteIdentifier(table),
			pq.QuoteLiteral(creationData.ShortName),
		)
		if _, err = q.Exec(query); err != nil {
			return fmt.Errorf("failed to create %s partition for board '%s': %w", table, creationData.ShortName, err)
		}
	}

	// Create the materialized view for board content previews.
	viewQuery := fmt.Sprintf(viewTmpl,
		ViewTableName(creationData.ShortName),
		s.cfg.Public.NLastMsg,
		pq.QuoteLiteral(creationData.ShortName),
	)
	if _, err = q.Exec(viewQuery); err != nil {
		return fmt.Errorf("failed to create materialized view for board '%s': %w", creationData.ShortName, err)
	}
	return nil
}

// deleteBoard contains the core DDL and DML logic for removing all database
// objects associated with a board. It must be executed within a transaction.
func (s *Storage) deleteBoard(q Querier, shortName domain.BoardShortName) error {
	// Get all file IDs from attachments in this board BEFORE dropping partitions
	// Note: Must do this before dropping attachments partition
	rows, err := q.Query(`
		SELECT DISTINCT file_id FROM attachments
		WHERE board = $1`,
		shortName)
	if err != nil {
		return fmt.Errorf("failed to get file IDs for board '%s': %w", shortName, err)
	}
	defer rows.Close()

	var fileIDs []int64
	for rows.Next() {
		var fileID int64
		if err := rows.Scan(&fileID); err != nil {
			return fmt.Errorf("failed to scan file ID: %w", err)
		}
		fileIDs = append(fileIDs, fileID)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating file IDs: %w", err)
	}

	// Drop the materialized view first, as it may depend on the tables to be dropped.
	if _, err := q.Exec(fmt.Sprintf("DROP MATERIALIZED VIEW IF EXISTS %s", ViewTableName(shortName))); err != nil {
		return fmt.Errorf("failed to drop view for board '%s': %w", shortName, err)
	}

	// Drop all table partitions associated with the board.
	// The CASCADE clause handles dependent objects like sequences.
	for _, table := range []string{"message_replies", "attachments", "messages", "threads"} {
		partition := PartitionName(shortName, table)
		if _, err := q.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", partition)); err != nil {
			return fmt.Errorf("failed to drop %s partition for board '%s': %w", table, shortName, err)
		}
	}

	// Finally, delete the board's metadata record. Foreign key constraints with
	// CASCADE will automatically delete all associated threads and messages.
	result, err := q.Exec("DELETE FROM boards WHERE short_name = $1", shortName)
	if err != nil {
		return fmt.Errorf("failed to delete board metadata for '%s': %w", shortName, err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return &internal_errors.ErrorWithStatusCode{
			Message: fmt.Sprintf("Board '%s' not found for deletion", shortName), StatusCode: http.StatusNotFound,
		}
	}

	// Delete file records in a single batch query
	// FK constraints will prevent deletion if files are still referenced elsewhere
	// This is best-effort - if it fails, the GC will clean up later
	if len(fileIDs) > 0 {
		_, err := q.Exec(`DELETE FROM files WHERE id = ANY($1)`, pq.Array(fileIDs))
		if err != nil {
			// Log warning but don't fail the operation
			logger.Log.Warn("failed to delete file records during board deletion",
				"board", shortName,
				"file_count", len(fileIDs),
				"error", err)
		}
	}

	return nil
}

// getBoard contains the core logic for fetching a board's content.
func (s *Storage) getBoard(q Querier, shortName domain.BoardShortName, page int) (domain.Board, error) {
	var metadata domain.BoardMetadata
	err := q.QueryRow(`
	       SELECT name, short_name, created_at, last_activity_at FROM boards WHERE short_name = $1`,
		shortName,
	).Scan(&metadata.Name, &metadata.ShortName, &metadata.CreatedAt, &metadata.LastActivityAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Board{}, &internal_errors.ErrorWithStatusCode{
				Message: fmt.Sprintf("Board '%s' not found", shortName), StatusCode: http.StatusNotFound,
			}
		}
		return domain.Board{}, fmt.Errorf("failed to fetch board metadata for '%s': %w", shortName, err)
	}

	// Fetch threads and their messages from the materialized view
	// ViewTableName returns an already quoted identifier
	rows, err := q.Query(
		fmt.Sprintf(`
            SELECT thread_title, message_count, last_bumped_at, thread_id, is_pinned,
                   msg_id, author_id, email_domain, author_is_admin, show_email_domain,
                   text, created_at
            FROM %s
            WHERE thread_order BETWEEN $1 * ($2 - 1) + 1 AND $1 * $2
            ORDER BY thread_order, msg_id
			`,
			ViewTableName(shortName),
		),
		s.cfg.Public.ThreadsPerPage,
		page,
	)
	if err != nil {
		return domain.Board{}, fmt.Errorf("failed to fetch threads for board '%s': %w", shortName, err)
	}
	defer rows.Close()

	type rowData struct {
		ThreadTitle       domain.ThreadTitle
		NMessages         int
		LastBumpTs        time.Time
		ThreadID          domain.ThreadId
		IsPinned          bool
		MsgID             domain.MsgId
		AuthorID          domain.UserId
		AuthorEmailDomain string
		AuthorIsAdmin     bool
		ShowEmailDomain   bool
		Text              domain.MsgText
		CreatedAt         time.Time
	}

	// Map for efficient message lookup when attaching replies and attachments
	// Key is (threadId, msgId) since msgId is per-thread
	idToMessage := make(map[MsgKey]*domain.Message)
	var messageKeys []MsgKey // Collect all message keys to fetch related data in bulk queries

	var threads []*domain.Thread
	var thread domain.Thread
	var currentThread domain.ThreadId = -1
	for rows.Next() {
		var row rowData
		if err := rows.Scan(
			&row.ThreadTitle, &row.NMessages, &row.LastBumpTs, &row.ThreadID, &row.IsPinned, &row.MsgID,
			&row.AuthorID, &row.AuthorEmailDomain, &row.AuthorIsAdmin, &row.ShowEmailDomain,
			&row.Text, &row.CreatedAt,
		); err != nil {
			return domain.Board{}, fmt.Errorf("failed to scan thread/message row: %w", err)
		}

		// rows are sorted by last_bumped_at and msg_id
		// so, if row.ThreadID != currentThread(basically previous row/rows thread_id)
		// that means new thread started, and we fully parsed previous thread
		// we need to add parsed thread to threads and create new thread object
		if currentThread != row.ThreadID {
			// Thread is not empty (can be empty if this is first row)
			if len(thread.Messages) > 0 {
				// Create a copy to avoid all pointers pointing to the same variable
				threadCopy := thread
				threads = append(threads, &threadCopy)
			}
			currentThread = row.ThreadID
			thread = domain.Thread{
				ThreadMetadata: domain.ThreadMetadata{
					Id:           row.ThreadID,
					Title:        row.ThreadTitle,
					Board:        shortName, // Board shortName from the outer scope
					MessageCount: row.NMessages,
					LastBumped:   row.LastBumpTs,
					IsPinned:     row.IsPinned,
				},
				Messages: []*domain.Message{},
			}
		}
		msg := &domain.Message{
			MessageMetadata: domain.MessageMetadata{
				Id: row.MsgID,
				Author: domain.User{
					Id:          row.AuthorID,
					EmailDomain: row.AuthorEmailDomain,
					Admin:       row.AuthorIsAdmin,
				},
				ShowEmailDomain: row.ShowEmailDomain,
				CreatedAt:       row.CreatedAt,
				ThreadId:        row.ThreadID,
				Board:           shortName,
				Replies:         domain.Replies{}, // Initialize empty replies slice
			},
			Text: row.Text,
		}
		thread.Messages = append(thread.Messages, msg)
		key := MsgKey{ThreadId: row.ThreadID, MsgId: row.MsgID}
		idToMessage[key] = msg
		messageKeys = append(messageKeys, key)
	}
	if err = rows.Err(); err != nil {
		return domain.Board{}, fmt.Errorf("error iterating thread/message rows: %w", err)
	}

	// Add the last thread if any threads were parsed
	if len(thread.Messages) > 0 {
		// Create a copy to avoid all pointers pointing to the same variable
		threadCopy := thread
		threads = append(threads, &threadCopy)
	}

	// Enrich parsed messages with replies
	if len(messageKeys) > 0 {
		if err := enrichMessagesWithReplies(q, shortName, messageKeys, idToMessage, s.cfg.Public.MessagesPerThreadPage); err != nil {
			return domain.Board{}, fmt.Errorf("failed to enrich replies for board page: %w", err)
		}
	}

	// Enrich parsed messages with attachments
	if len(messageKeys) > 0 {
		if err := enrichMessagesWithAttachments(q, shortName, messageKeys, idToMessage); err != nil {
			return domain.Board{}, fmt.Errorf("failed to enrich attachments for board page: %w", err)
		}
	}

	return domain.Board{
		BoardMetadata: metadata,
		Threads:       threads,
	}, nil
}

// getBoards contains the core logic for fetching all board metadata.
func (s *Storage) getBoards(q Querier) ([]domain.Board, error) {
	var boards []domain.Board
	rows, err := q.Query(`
	SELECT
		name, short_name, created_at, last_activity_at
	FROM boards
	ORDER BY short_name
	`) // Querying all fields that constitute BoardMetadata
	if err != nil {
		return nil, fmt.Errorf("failed to query boards: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var boardMeta domain.BoardMetadata
		err = rows.Scan(
			&boardMeta.Name,
			&boardMeta.ShortName,
			&boardMeta.CreatedAt,
			&boardMeta.LastActivityAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan board metadata: %w", err)
		}
		boards = append(boards, domain.Board{BoardMetadata: boardMeta, Threads: nil}) // Threads are not fetched here
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating board rows: %w", err)
	}

	// Enrich boards with permissions (corporate vs public board distinction)
	if err := enrichBoardsWithPermissions(q, boards); err != nil {
		return nil, err
	}

	return boards, nil
}

// getBoardsByUser contains the core logic for fetching board metadata visible to a user.
func (s *Storage) getBoardsByUser(q Querier, user domain.User) ([]domain.Board, error) {
	if user.Admin {
		return s.getBoards(q)
	}

	var boards []domain.Board
	var err error
	// restrict boards for non-admins
	userEmailDomain, domainErr := user.GetEmailDomain()
	if domainErr != nil {
		return nil, fmt.Errorf("could not determine user email domain for user %d: %w", user.Id, domainErr)
	}
	// Use UNION ALL pattern for better query performance:
	// 1. Public boards (no permissions set)
	// 2. User's permitted boards (email domain matches)
	rows, err := q.Query(`
		SELECT
			b.name, b.short_name, b.created_at, b.last_activity_at
		FROM boards b
		WHERE NOT EXISTS (
			SELECT 1 FROM board_permissions
			WHERE board_short_name = b.short_name
		)
		UNION ALL
		SELECT
			b.name, b.short_name, b.created_at, b.last_activity_at
		FROM boards b
		INNER JOIN board_permissions bp ON b.short_name = bp.board_short_name
		WHERE bp.allowed_email_domain = $1
		ORDER BY short_name
		`, userEmailDomain)
	if err != nil {
		return nil, fmt.Errorf("failed to query boards for user: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var boardMeta domain.BoardMetadata
		err = rows.Scan(
			&boardMeta.Name,
			&boardMeta.ShortName,
			&boardMeta.CreatedAt,
			&boardMeta.LastActivityAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan board data for user: %w", err)
		}
		boards = append(boards, domain.Board{BoardMetadata: boardMeta, Threads: nil}) // Threads are not fetched here
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating board rows for user: %w", err)
	}

	// Enrich boards with permissions (corporate vs public board distinction)
	if err := enrichBoardsWithPermissions(q, boards); err != nil {
		return nil, err
	}

	return boards, nil
}

// getActiveBoards contains the core logic for finding boards with recent activity.
func (s *Storage) getActiveBoards(q Querier, interval time.Duration) ([]domain.Board, error) {
	rows, err := q.Query(`
	SELECT 
		short_name 
	FROM boards
	WHERE EXTRACT(EPOCH FROM ((now() at time zone 'utc') - last_activity_at)) < $1`,
		interval.Seconds(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query active boards: %w", err)
	}
	defer rows.Close()

	var boards []domain.Board
	for rows.Next() {
		var board domain.Board
		if err = rows.Scan(&board.ShortName); err != nil {
			return nil, fmt.Errorf("failed to scan active board short_name: %w", err)
		}
		boards = append(boards, board)
	}
	return boards, rows.Err()
}

// threadCount contains the core logic for counting threads on a board.
func (s *Storage) threadCount(q Querier, board domain.BoardShortName) (int, error) {
	var count int
	err := q.QueryRow(`SELECT count(*) as count FROM threads WHERE board = $1`, board).Scan(&count)
	if err != nil {
		return -1, fmt.Errorf("failed to count threads for board '%s': %w", board, err)
	}
	return count, nil
}

// lastThreadId contains the core logic for finding the least recently bumped thread.
func (s *Storage) lastThreadId(q Querier, board domain.BoardShortName) (domain.MsgId, error) {
	var id int64
	err := q.QueryRow(`
	SELECT id FROM threads
	WHERE board = $1 AND is_pinned = FALSE
	ORDER BY last_bumped_at ASC, id LIMIT 1`,
		board,
	).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return -1, &internal_errors.ErrorWithStatusCode{
				Message: fmt.Sprintf("No non-pinned threads found on board '%s'", board), StatusCode: http.StatusNotFound,
			}
		}
		return -1, fmt.Errorf("failed to get last thread ID for board '%s': %w", board, err)
	}
	return id, nil
}
