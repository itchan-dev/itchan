package pg

import (
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

func (s *Storage) CreateBoard(creationData domain.BoardCreationData) error {
	if creationData.AllowedEmails != nil && len(*creationData.AllowedEmails) == 0 {
		return fmt.Errorf("%w: allowed_emails cannot be empty", emptyAllowedEmailsError)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert board metadata
	_, err = tx.Exec(`
        INSERT INTO boards (name, short_name, allowed_emails)
        VALUES ($1, $2, $3)`,
		creationData.Name,
		creationData.ShortName,
		creationData.AllowedEmails,
	)
	if err != nil {
		// Check for unique constraint violation for short_name specifically if pq driver allows
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" { // Unique violation
			return fmt.Errorf("failed to insert board, possibly duplicate short_name '%s': %w", creationData.ShortName, err)
		}
		return fmt.Errorf("failed to insert board: %w", err)
	}

	// Create partitions for threads and messages
	for _, table := range []string{"threads", "messages"} {
		seqName := fmt.Sprintf("%s_id_seq_%s", table, creationData.ShortName)
		// PartitionName returns an already quoted identifier
		query := fmt.Sprintf(partitionTmpl,
			pq.QuoteIdentifier(seqName),
			PartitionName(creationData.ShortName, table),
			pq.QuoteIdentifier(table), // Parent table name
			pq.QuoteLiteral(seqName),
			pq.QuoteLiteral(creationData.ShortName),
		)
		if _, err = tx.Exec(query); err != nil {
			return fmt.Errorf("failed to create %s partition for board '%s': %w", table, creationData.ShortName, err)
		}
	}

	// Create materialized view for board preview
	// ViewTableName returns an already quoted identifier
	viewQuery := fmt.Sprintf(viewTmpl,
		ViewTableName(creationData.ShortName),
		s.cfg.Public.NLastMsg,
		pq.QuoteLiteral(creationData.ShortName),
	)
	if _, err = tx.Exec(viewQuery); err != nil {
		return fmt.Errorf("failed to create materialized view for board '%s': %w", creationData.ShortName, err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func (s *Storage) GetBoard(shortName domain.BoardShortName, page int) (domain.Board, error) {
	var metadata domain.BoardMetadata
	err := s.db.QueryRow(`
        SELECT name, short_name, allowed_emails, created, last_activity
        FROM boards
        WHERE short_name = $1`,
		shortName,
	).Scan(
		&metadata.Name,
		&metadata.ShortName,
		&metadata.AllowedEmails,
		&metadata.CreatedAt,
		&metadata.LastActivity,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Board{}, &internal_errors.ErrorWithStatusCode{
				Message:    fmt.Sprintf("Board '%s' not found", shortName),
				StatusCode: http.StatusNotFound,
			}
		}
		return domain.Board{}, fmt.Errorf("failed to fetch board metadata for '%s': %w", shortName, err)
	}

	// Fetch threads and their messages from the materialized view
	// ViewTableName returns an already quoted identifier
	rows, err := s.db.Query(
		fmt.Sprintf(`
            SELECT thread_title, num_replies, last_bump_ts, thread_id,
                   msg_id, author_id, text, created, attachments, op
            FROM %s
            WHERE thread_order BETWEEN $1 * ($2 - 1) + 1 AND $1 * $2
            ORDER BY thread_order, created`, // `created` for message order within thread
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
	type rawThreadMessageData struct {
		ThreadTitle domain.ThreadTitle
		NReplies    int
		LastBumpTs  time.Time
		ThreadID    domain.ThreadId
		MsgID       domain.MsgId
		AuthorID    domain.UserId
		Text        domain.MsgText
		Created     time.Time
		Attachments *domain.Attachments
		Op          bool
	}
	var allRowData []rawThreadMessageData

	for rows.Next() {
		var rtd rawThreadMessageData
		if err := rows.Scan(
			&rtd.ThreadTitle, &rtd.NReplies, &rtd.LastBumpTs, &rtd.ThreadID,
			&rtd.MsgID, &rtd.AuthorID, &rtd.Text, &rtd.Created, &rtd.Attachments, &rtd.Op,
		); err != nil {
			return domain.Board{}, fmt.Errorf("failed to scan thread/message row: %w", err)
		}
		allRowData = append(allRowData, rtd)
	}
	if err = rows.Err(); err != nil {
		return domain.Board{}, fmt.Errorf("error iterating thread/message rows: %w", err)
	}

	// Process allRowData to build domain.Board.Threads
	var threads []domain.Thread
	// Map threadID to its index in the `threads` slice to efficiently append messages
	threadIndexMap := make(map[domain.ThreadId]int)

	for _, rtd := range allRowData {
		idx, exists := threadIndexMap[rtd.ThreadID]
		if !exists {
			// This is the first time we're seeing this thread_id from the view's current page
			newThread := domain.Thread{
				ThreadMetadata: domain.ThreadMetadata{
					Id:         rtd.ThreadID,
					Title:      rtd.ThreadTitle,
					Board:      shortName, // Board shortName from the outer scope
					NumReplies: rtd.NReplies,
					LastBumped: rtd.LastBumpTs,
					// IsSticky is not in the view, if needed, it has to be added or fetched separately
				},
				Messages: []domain.Message{},
			}
			threads = append(threads, newThread)
			idx = len(threads) - 1 // Get the index of the newly added thread
			threadIndexMap[rtd.ThreadID] = idx
		}

		// Append the message to the correct thread
		// The view is ordered by thread_order, then created, so OP should be first for a thread.
		threads[idx].Messages = append(threads[idx].Messages, domain.Message{
			MessageMetadata: domain.MessageMetadata{
				Id:        rtd.MsgID,
				Author:    domain.User{Id: rtd.AuthorID},
				CreatedAt: rtd.Created,
				ThreadId:  rtd.ThreadID,
				Op:        rtd.Op,
				Board:     shortName, // Board shortName from the outer scope
				// Ordinal and ModifiedAt are not in the view
			},
			Text:        rtd.Text,
			Attachments: rtd.Attachments,
		})
	}

	return domain.Board{
		BoardMetadata: metadata,
		Threads:       threads,
	}, nil
}

func (s *Storage) DeleteBoard(shortName domain.BoardShortName) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop materialized view
	// ViewTableName returns an already quoted identifier
	if _, err := tx.Exec(fmt.Sprintf("DROP MATERIALIZED VIEW IF EXISTS %s", ViewTableName(shortName))); err != nil {
		return fmt.Errorf("failed to drop view for board '%s': %w", shortName, err)
	}

	// Drop partitions
	for _, table := range []string{"messages", "threads"} {
		// PartitionName returns an already quoted identifier
		partition := PartitionName(shortName, table)
		if _, err := tx.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", partition)); err != nil {
			return fmt.Errorf("failed to drop %s partition for board '%s': %w", table, shortName, err)
		}
	}

	// Delete board metadata
	result, err := tx.Exec("DELETE FROM boards WHERE short_name = $1", shortName)
	if err != nil {
		return fmt.Errorf("failed to delete board '%s': %w", shortName, err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return &internal_errors.ErrorWithStatusCode{
			Message:    fmt.Sprintf("Board '%s' not found for deletion", shortName),
			StatusCode: http.StatusNotFound,
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func (s *Storage) GetBoards() ([]domain.Board, error) {
	var boards []domain.Board
	rows, err := s.db.Query(`
	SELECT
		name, short_name, allowed_emails, created, last_activity
	FROM boards
	ORDER BY created
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
			&boardMeta.AllowedEmails,
			&boardMeta.CreatedAt,
			&boardMeta.LastActivity,
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

func (s *Storage) GetBoardsByUser(user domain.User) ([]domain.Board, error) {
	if user.Admin { // return all boards if user = admin
		return s.GetBoards()
	}
	var boards []domain.Board
	var err error
	var rows *sql.Rows

	// restrict boards for non-admins
	userEmailDomain, domainErr := user.EmailDomain()
	if domainErr != nil {
		return nil, fmt.Errorf("could not determine user email domain for '%s': %w", user.Email, domainErr)
	}
	rows, err = s.db.Query(`
		SELECT
			name, short_name, allowed_emails, created, last_activity
		FROM boards
		WHERE (allowed_emails IS NULL) OR ($1 = ANY(allowed_emails))
		ORDER BY created
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
			&boardMeta.AllowedEmails,
			&boardMeta.CreatedAt,
			&boardMeta.LastActivity,
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

// Boards that had new messages at last `interval` time
func (s *Storage) GetActiveBoards(interval time.Duration) ([]domain.Board, error) {
	var boards []domain.Board
	rows, err := s.db.Query(`
	SELECT
		short_name
	FROM boards
	WHERE EXTRACT(EPOCH FROM ((now() at time zone 'utc') - last_activity)) < $1
	`, interval.Seconds())
	if err != nil {
		return nil, fmt.Errorf("failed to query active boards: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var board domain.Board // Only BoardMetadata.ShortName is populated
		if err = rows.Scan(&board.ShortName); err != nil {
			return nil, fmt.Errorf("failed to scan active board short_name: %w", err)
		}
		boards = append(boards, board)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating active board rows: %w", err)
	}
	return boards, nil
}

func (s *Storage) ThreadCount(board domain.BoardShortName) (int, error) {
	var count int
	err := s.db.QueryRow(`
	SELECT 
		count(*) as count
	FROM threads 
	WHERE board = $1
	`, board).Scan(&count)
	if err != nil {
		return -1, fmt.Errorf("failed to count threads for board '%s': %w", board, err)
	}
	return count, nil
}

func (s *Storage) LastThreadId(board domain.BoardShortName) (domain.MsgId, error) {
	var id int64
	err := s.db.QueryRow(`
	SELECT 
		id
	FROM threads 
	WHERE 
		board = $1 
		AND is_sticky = FALSE 
	ORDER BY last_bump_ts ASC 
	LIMIT 1
	`, board).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return -1, &internal_errors.ErrorWithStatusCode{
				Message:    fmt.Sprintf("No non-sticky threads found on board '%s'", board),
				StatusCode: http.StatusNotFound,
			}
		}
		return -1, fmt.Errorf("failed to get last thread ID for board '%s': %w", board, err)
	}
	return id, nil
}
