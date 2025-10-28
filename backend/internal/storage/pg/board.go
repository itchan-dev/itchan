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

	"github.com/lib/pq"
)

var emptyAllowedEmailsError = errors.New("allowedEmails should be either nil or not empty")

//go:embed templates/board_view_template.sql
var viewTmpl string

//go:embed templates/partition_template.sql
var partitionTmpl string

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
// bumped non-sticky thread, often used for pruning.
func (s *Storage) LastThreadId(board domain.BoardShortName) (domain.MsgId, error) {
	return s.lastThreadId(s.db, board)
}

// GetBoardsWithPermissions returns a map of board short names to their allowed email domains.
// Returns nil for boards without restrictions (public boards).
func (s *Storage) GetBoardsWithPermissions() (map[string][]string, error) {
	return s.getBoardsWithPermissions(s.db)
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

	// Create partitions for all partitioned tables.
	for _, table := range []string{"threads", "messages", "attachments"} {
		var query string
		// Tables requiring a sequence for their partitioned ID need the partition template.
		seqName := fmt.Sprintf("%s_id_seq_%s", table, creationData.ShortName)
		query = fmt.Sprintf(partitionTmpl,
			pq.QuoteIdentifier(seqName),
			PartitionName(creationData.ShortName, table),
			pq.QuoteIdentifier(table),
			pq.QuoteLiteral(seqName),
			pq.QuoteLiteral(creationData.ShortName),
		)
		if _, err = q.Exec(query); err != nil {
			return fmt.Errorf("failed to create %s partition for board '%s': %w", table, creationData.ShortName, err)
		}
	}

	// Tables without a sequence have a simpler creation statement.
	table := "message_replies"
	query := fmt.Sprintf(`CREATE TABLE %s PARTITION OF %s FOR VALUES IN (%s);`,
		PartitionName(creationData.ShortName, table),
		pq.QuoteIdentifier(table),
		pq.QuoteLiteral(creationData.ShortName),
	)
	if _, err = q.Exec(query); err != nil {
		return fmt.Errorf("failed to create %s partition for board '%s': %w", table, creationData.ShortName, err)
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
            SELECT thread_title, message_count, last_bumped_at, thread_id,
                   msg_id, author_id, text, created_at, is_op, ordinal
            FROM %s
            WHERE thread_order BETWEEN $1 * ($2 - 1) + 1 AND $1 * $2
            ORDER BY thread_order, ordinal -- at first, order by thread then by message inside thread
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

	// Temporary structure to hold flat row data from the view
	type rowData struct {
		ThreadTitle domain.ThreadTitle
		NMessages   int
		LastBumpTs  time.Time
		ThreadID    domain.ThreadId
		MsgID       domain.MsgId
		AuthorID    domain.UserId
		Text        domain.MsgText
		CreatedAt   time.Time
		IsOp        bool
		Ordinal     int
	}

	idToMessage := make(map[domain.MsgId]*domain.Message) // Map for efficient message lookup when attaching replies and attachments
	var messageIds []domain.MsgId                         // Collect all message IDs to fetch related data in bulk queries

	var threads []*domain.Thread
	var thread domain.Thread
	var currentThread domain.ThreadId = -1
	for rows.Next() {
		var row rowData
		if err := rows.Scan(
			&row.ThreadTitle, &row.NMessages, &row.LastBumpTs, &row.ThreadID, &row.MsgID,
			&row.AuthorID, &row.Text, &row.CreatedAt, &row.IsOp, &row.Ordinal,
		); err != nil {
			return domain.Board{}, fmt.Errorf("failed to scan thread/message row: %w", err)
		}
		// rows are sorted by last_bumped_at and reply_number
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
				},
				Messages: []*domain.Message{},
			}
		}
		msg := &domain.Message{
			MessageMetadata: domain.MessageMetadata{
				Id:        row.MsgID,
				Author:    domain.User{Id: row.AuthorID},
				CreatedAt: row.CreatedAt,
				ThreadId:  row.ThreadID,
				Op:        row.IsOp,
				Board:     shortName,
				Ordinal:   row.Ordinal,
				Replies:   domain.Replies{}, // Initialize empty replies slice
			},
			Text: row.Text,
		}
		thread.Messages = append(thread.Messages, msg)
		idToMessage[msg.Id] = msg
		messageIds = append(messageIds, msg.Id) // Uniqueness ensured by unique index on board materialized view
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
	if len(messageIds) > 0 {
		rows, err := q.Query(`
		SELECT 
			sender_message_id,
			sender_thread_id,
			receiver_message_id,
			receiver_thread_id,
			created_at
		FROM message_replies
		WHERE board = $1 AND receiver_message_id = ANY($2)
		ORDER BY created_at
		`, shortName, pq.Array(messageIds))
		if err != nil {
			return domain.Board{}, fmt.Errorf("failed to fetch replies for board page: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var reply domain.Reply
			if err := rows.Scan(&reply.From, &reply.FromThreadId, &reply.To, &reply.ToThreadId, &reply.CreatedAt); err != nil {
				return domain.Board{}, fmt.Errorf("failed to scan reply row for board page: %w", err)
			}

			reply.Board = shortName // Set the board field

			if msg, ok := idToMessage[reply.To]; ok {
				msg.Replies = append(msg.Replies, &reply)
			}
		}
		if err = rows.Err(); err != nil {
			return domain.Board{}, fmt.Errorf("replies iteration error for board page: %w", err)
		}
	}

	// Enrich parsed messages with attachments
	if len(messageIds) > 0 {
		rows, err := q.Query(`
        SELECT a.id, a.board, a.message_id, a.file_id,
               f.file_path, f.original_filename, f.file_size_bytes, f.mime_type, f.image_width, f.image_height
        FROM attachments a
        JOIN files f ON a.file_id = f.id
        WHERE a.board = $1 AND a.message_id = ANY($2)
        ORDER BY a.id
		`, shortName, pq.Array(messageIds))
		if err != nil {
			return domain.Board{}, fmt.Errorf("failed to fetch attachments for board page: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var attachment domain.Attachment
			var file domain.File
			if err := rows.Scan(
				&attachment.Id, &attachment.Board, &attachment.MessageId, &attachment.FileId,
				&file.FilePath, &file.OriginalFilename, &file.FileSizeBytes, &file.MimeType, &file.ImageWidth, &file.ImageHeight,
			); err != nil {
				return domain.Board{}, fmt.Errorf("failed to scan attachment row for board page: %w", err)
			}
			attachment.File = &file
			if msg, ok := idToMessage[attachment.MessageId]; ok {
				msg.Attachments = append(msg.Attachments, &attachment)
			}
		}
		if err = rows.Err(); err != nil {
			return domain.Board{}, fmt.Errorf("attachments iteration error for board page: %w", err)
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
	userEmailDomain, domainErr := user.EmailDomain()
	if domainErr != nil {
		return nil, fmt.Errorf("could not determine user email domain for '%s': %w", user.Email, domainErr)
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
	WHERE board = $1 AND is_sticky = FALSE
	ORDER BY last_bumped_at ASC, id LIMIT 1`,
		board,
	).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return -1, &internal_errors.ErrorWithStatusCode{
				Message: fmt.Sprintf("No non-sticky threads found on board '%s'", board), StatusCode: http.StatusNotFound,
			}
		}
		return -1, fmt.Errorf("failed to get last thread ID for board '%s': %w", board, err)
	}
	return id, nil
}

// getBoardsWithPermissions retrieves all board permissions and returns them as a map.
// For boards without any permissions (public boards), the key will not be present in the map.
func (s *Storage) getBoardsWithPermissions(q Querier) (map[string][]string, error) {
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
