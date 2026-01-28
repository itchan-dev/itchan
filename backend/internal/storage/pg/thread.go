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
	"github.com/itchan-dev/itchan/shared/utils"
)

// =========================================================================
// Public Methods (satisfy the service.ThreadStorage interface)
// =========================================================================

// CreateThread is the public entry point for creating a new thread record.
// It ONLY creates the thread metadata - the OP message should be created
// separately by the service layer. This maintains proper separation of concerns.
func (s *Storage) CreateThread(creationData domain.ThreadCreationData) (domain.ThreadId, time.Time, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var threadID domain.ThreadId
	var createdAt time.Time

	err := s.withTx(ctx, func(tx *sql.Tx) error {
		var err error
		threadID, createdAt, err = s.createThread(tx, creationData)
		return err
	})
	return threadID, createdAt, err
}

// GetThread is the public entry point for fetching a full thread, including all of its
// messages, replies, and attachments. It decides whether to use the optimized single-page
// fetch or the paginated fetch based on thread size.
// The page parameter controls pagination (1-based). Page 0 or 1 returns the first page.
func (s *Storage) GetThread(board domain.BoardShortName, id domain.ThreadId, page int) (domain.Thread, error) {
	return s.getThread(s.db, board, id, page)
}

// DeleteThread is the public entry point for deleting a thread. It wraps the core
// deletion logic in a transaction to ensure atomicity. The database schema's
// foreign key constraints will cascade the delete from the thread to all of its
// contained messages, attachments, and replies.
func (s *Storage) DeleteThread(board domain.BoardShortName, id domain.ThreadId) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.withTx(ctx, func(tx *sql.Tx) error {
		return s.deleteThread(tx, board, id)
	})
}

// TogglePinnedStatus is the public entry point for toggling a thread's pinned status.
// It wraps the update in a transaction and returns the new pinned status.
func (s *Storage) TogglePinnedStatus(board domain.BoardShortName, threadId domain.ThreadId) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var newStatus bool
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		var err error
		newStatus, err = s.togglePinnedStatus(tx, board, threadId)
		return err
	})
	return newStatus, err
}

// =========================================================================
// Internal Methods (Core Database Logic)
// These methods accept a Querier and are transaction-agnostic.
// =========================================================================

// getThread contains the core logic for fetching a thread. It accepts a Querier to support
// both direct database access and transactional contexts (used by tests).
// It decides whether to use the optimized single-page fetch or paginated fetch based on thread size.
func (s *Storage) getThread(q Querier, board domain.BoardShortName, id domain.ThreadId, page int) (domain.Thread, error) {
	if page < 1 {
		page = 1
	}

	var metadata domain.ThreadMetadata
	err := q.QueryRow(`
		SELECT
			id, title, board, message_count, last_bumped_at, is_pinned
		FROM threads
		WHERE board = $1 AND id = $2`,
		board, id,
	).Scan(
		&metadata.Id, &metadata.Title, &metadata.Board,
		&metadata.MessageCount, &metadata.LastBumped, &metadata.IsPinned,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Thread{}, &internal_errors.ErrorWithStatusCode{Message: "Thread not found", StatusCode: http.StatusNotFound}
		}
		return domain.Thread{}, fmt.Errorf("failed to fetch thread metadata: %w", err)
	}

	messagesPerPage := s.cfg.Public.MessagesPerThreadPage

	if metadata.MessageCount <= messagesPerPage {
		return s.getThreadSinglePage(q, metadata, board, id)
	}

	return s.getThreadPaginated(q, metadata, board, id, page, messagesPerPage)
}

// createThread handles the specific database operation of inserting a new record
// into the `threads` table. It's unexported and designed to be called within
// a transaction managed by a public method. It returns the new thread's ID and
// creation timestamp, which are needed by the caller to create the OP message.
func (s *Storage) createThread(q Querier, creationData domain.ThreadCreationData) (domain.ThreadId, time.Time, error) {
	// First, verify that the target board actually exists.
	var board domain.BoardShortName
	err := q.QueryRow(
		"SELECT short_name FROM boards WHERE short_name = $1",
		creationData.Board,
	).Scan(&board)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return -1, time.Time{}, &internal_errors.ErrorWithStatusCode{
				Message: "Board not found", StatusCode: http.StatusNotFound,
			}
		}
		return -1, time.Time{}, fmt.Errorf("failed to validate board for thread creation: %w", err)
	}

	// Determine the creation timestamp for the thread and its OP message.
	// Use the provided timestamp if available (useful for testing or migrations),
	// otherwise generate the current UTC timestamp rounded to microseconds for consistency.
	var createdAt time.Time
	if creationData.OpMessage.CreatedAt != nil {
		createdAt = *creationData.OpMessage.CreatedAt
	} else {
		createdAt = time.Now().UTC().Round(time.Microsecond)
	}

	// Insert the thread into its board-specific partition.
	var id domain.ThreadId
	var createdTs time.Time
	partitionName := PartitionName(creationData.Board, "threads")
	err = q.QueryRow(
		fmt.Sprintf(`
	           INSERT INTO %s (title, board, is_pinned, created_at)
	           VALUES ($1, $2, $3, $4)
	           RETURNING id, created_at
	       `, partitionName),
		creationData.Title,
		creationData.Board,
		creationData.IsPinned,
		createdAt,
	).Scan(&id, &createdTs)
	if err != nil {
		return -1, time.Time{}, fmt.Errorf("failed to insert thread record: %w", err)
	}

	return id, createdTs, nil
}

// getThreadSinglePage fetches all messages, replies, and attachments for a thread
// using thread-level queries. This is faster for small threads (95% of cases)
// because it avoids the overhead of building message key arrays.
func (s *Storage) getThreadSinglePage(q Querier, metadata domain.ThreadMetadata, board domain.BoardShortName, id domain.ThreadId) (domain.Thread, error) {
	var messages []*domain.Message
	msgIDMap := make(map[domain.MsgId]*domain.Message)

	// Fetch all messages for the thread
	msgRows, err := q.Query(`
		SELECT
			m.id, m.author_id, u.email_domain, m.text, m.created_at, m.thread_id,
			m.updated_at, m.board, u.is_admin
		FROM messages m
		JOIN users u ON m.author_id = u.id
		WHERE m.board = $1 AND m.thread_id = $2
		ORDER BY m.id`,
		board, id,
	)
	if err != nil {
		return domain.Thread{}, fmt.Errorf("failed to fetch thread messages: %w", err)
	}
	defer msgRows.Close()

	for msgRows.Next() {
		var msg domain.Message
		if err := msgRows.Scan(
			&msg.Id, &msg.Author.Id, &msg.Author.EmailDomain, &msg.Text, &msg.CreatedAt,
			&msg.ThreadId, &msg.ModifiedAt, &msg.Board, &msg.Author.Admin,
		); err != nil {
			return domain.Thread{}, fmt.Errorf("failed to scan message row: %w", err)
		}
		msg.Page = 1 // Single page thread
		messages = append(messages, &msg)
		msgIDMap[msg.Id] = &msg
	}
	if err = msgRows.Err(); err != nil {
		return domain.Thread{}, fmt.Errorf("error iterating message rows: %w", err)
	}

	// Fetch all reply relationships for the entire thread
	replyRows, err := q.Query(`
		SELECT
			mr.board, mr.sender_message_id, mr.sender_thread_id, mr.receiver_message_id, mr.receiver_thread_id,
			mr.created_at
		FROM message_replies mr
		WHERE mr.board = $1 AND mr.receiver_thread_id = $2
		ORDER BY mr.created_at`,
		board, id,
	)
	if err != nil {
		return domain.Thread{}, fmt.Errorf("failed to fetch thread replies: %w", err)
	}
	defer replyRows.Close()
	for replyRows.Next() {
		var reply domain.Reply
		if err := replyRows.Scan(&reply.Board, &reply.From, &reply.FromThreadId, &reply.To, &reply.ToThreadId, &reply.CreatedAt); err != nil {
			return domain.Thread{}, fmt.Errorf("failed to scan reply row: %w", err)
		}
		reply.FromPage = 1 // Single page thread
		if msg, ok := msgIDMap[reply.To]; ok {
			msg.Replies = append(msg.Replies, &reply)
		}
	}

	// Fetch all attachments for the entire thread
	attachRows, err := q.Query(`
		SELECT
			a.id, a.board, a.thread_id, a.message_id, a.file_id,
			f.file_path, f.filename, f.original_filename, f.file_size_bytes, f.mime_type, f.original_mime_type, f.image_width, f.image_height, f.thumbnail_path
		FROM attachments a
		JOIN files f ON a.file_id = f.id
		WHERE a.board = $1 AND a.thread_id = $2
		ORDER BY a.id`,
		board, id,
	)
	if err != nil {
		return domain.Thread{}, fmt.Errorf("failed to fetch thread attachments: %w", err)
	}
	defer attachRows.Close()
	for attachRows.Next() {
		var attachment domain.Attachment
		var file domain.File
		if err := attachRows.Scan(
			&attachment.Id, &attachment.Board, &attachment.ThreadId, &attachment.MessageId, &attachment.FileId,
			&file.FilePath, &file.Filename, &file.OriginalFilename, &file.SizeBytes, &file.MimeType, &file.OriginalMimeType, &file.ImageWidth, &file.ImageHeight, &file.ThumbnailPath,
		); err != nil {
			return domain.Thread{}, fmt.Errorf("failed to scan attachment row: %w", err)
		}
		attachment.File = &file
		if msg, ok := msgIDMap[attachment.MessageId]; ok {
			msg.Attachments = append(msg.Attachments, &attachment)
		}
	}

	return domain.Thread{
		ThreadMetadata: metadata,
		Messages:       messages,
		Pagination: &domain.ThreadPagination{
			CurrentPage: 1,
			TotalPages:  1,
			TotalCount:  metadata.MessageCount,
		},
	}, nil
}

// getThreadPaginated fetches messages for a specific page and enriches only those
// messages with replies and attachments. This is more efficient for large threads
// as it avoids fetching data for messages not on the current page.
func (s *Storage) getThreadPaginated(q Querier, metadata domain.ThreadMetadata, board domain.BoardShortName, id domain.ThreadId, page int, messagesPerPage int) (domain.Thread, error) {
	offset := (page - 1) * messagesPerPage

	var messages []*domain.Message
	idToMessage := make(map[MsgKey]*domain.Message)
	var messageKeys []MsgKey

	// For pages > 1, first fetch the OP message separately so it's always visible
	if page > 1 {
		opRow := q.QueryRow(`
			SELECT
				m.id, m.author_id, u.email_domain, m.text, m.created_at, m.thread_id,
				m.updated_at, m.board, u.is_admin
			FROM messages m
			JOIN users u ON m.author_id = u.id
			WHERE m.board = $1 AND m.thread_id = $2 AND m.id = 1`,
			board, id,
		)
		var opMsg domain.Message
		if err := opRow.Scan(
			&opMsg.Id, &opMsg.Author.Id, &opMsg.Author.EmailDomain, &opMsg.Text, &opMsg.CreatedAt,
			&opMsg.ThreadId, &opMsg.ModifiedAt, &opMsg.Board, &opMsg.Author.Admin,
		); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return domain.Thread{}, fmt.Errorf("failed to fetch OP message: %w", err)
		} else if err == nil {
			opMsg.Page = 1
			opMsg.Replies = domain.Replies{}
			opMsg.Attachments = domain.Attachments{}
			messages = append(messages, &opMsg)
			key := MsgKey{ThreadId: id, MsgId: opMsg.Id}
			idToMessage[key] = &opMsg
			messageKeys = append(messageKeys, key)
		}
	}

	// Fetch paginated messages for the thread
	msgRows, err := q.Query(`
		SELECT
			m.id, m.author_id, u.email_domain, m.text, m.created_at, m.thread_id,
			m.updated_at, m.board, u.is_admin
		FROM messages m
		JOIN users u ON m.author_id = u.id
		WHERE m.board = $1 AND m.thread_id = $2
		ORDER BY m.id
		LIMIT $3 OFFSET $4`,
		board, id, messagesPerPage, offset,
	)
	if err != nil {
		return domain.Thread{}, fmt.Errorf("failed to fetch thread messages: %w", err)
	}
	defer msgRows.Close()

	for msgRows.Next() {
		var msg domain.Message
		if err := msgRows.Scan(
			&msg.Id, &msg.Author.Id, &msg.Author.EmailDomain, &msg.Text, &msg.CreatedAt,
			&msg.ThreadId, &msg.ModifiedAt, &msg.Board, &msg.Author.Admin,
		); err != nil {
			return domain.Thread{}, fmt.Errorf("failed to scan message row: %w", err)
		}
		msg.Page = utils.CalculatePage(int(msg.Id), messagesPerPage)
		msg.Replies = domain.Replies{}
		msg.Attachments = domain.Attachments{}
		messages = append(messages, &msg)
		key := MsgKey{ThreadId: id, MsgId: msg.Id}
		idToMessage[key] = &msg
		messageKeys = append(messageKeys, key)
	}
	if err = msgRows.Err(); err != nil {
		return domain.Thread{}, fmt.Errorf("error iterating message rows: %w", err)
	}

	// Enrich only the messages on this page using the shared enrichment functions
	if len(messageKeys) > 0 {
		if err := enrichMessagesWithReplies(q, board, messageKeys, idToMessage, messagesPerPage); err != nil {
			return domain.Thread{}, fmt.Errorf("failed to enrich replies for thread page: %w", err)
		}
		if err := enrichMessagesWithAttachments(q, board, messageKeys, idToMessage); err != nil {
			return domain.Thread{}, fmt.Errorf("failed to enrich attachments for thread page: %w", err)
		}
	}

	// Calculate pagination info
	totalPages := max((metadata.MessageCount+messagesPerPage-1)/messagesPerPage, 1)

	return domain.Thread{
		ThreadMetadata: metadata,
		Messages:       messages,
		Pagination: &domain.ThreadPagination{
			CurrentPage: page,
			TotalPages:  totalPages,
			TotalCount:  metadata.MessageCount,
		},
	}, nil
}

// deleteThread contains the core logic for removing a thread record.
func (s *Storage) deleteThread(q Querier, board domain.BoardShortName, id domain.ThreadId) error {
	// Update the board's last_activity timestamp.
	result, err := q.Exec(`
        UPDATE boards SET last_activity_at = NOW() AT TIME ZONE 'utc'
        WHERE short_name = $1`,
		board,
	)
	if err != nil {
		return fmt.Errorf("failed to update board activity on thread delete: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return &internal_errors.ErrorWithStatusCode{Message: "Board not found", StatusCode: http.StatusNotFound}
	}

	// Delete the thread from its partition. ON DELETE CASCADE will handle all child messages.
	result, err = q.Exec("DELETE FROM threads WHERE board = $1 AND id = $2", board, id)
	if err != nil {
		return fmt.Errorf("failed to delete thread: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return &internal_errors.ErrorWithStatusCode{Message: "Thread not found", StatusCode: http.StatusNotFound}
	}

	return nil
}

// togglePinnedStatus contains the core logic for toggling a thread's pinned status.
// It uses NOT is_pinned to atomically toggle the value and returns the new status.
func (s *Storage) togglePinnedStatus(q Querier, board domain.BoardShortName, threadId domain.ThreadId) (bool, error) {
	// Update the board's last_activity timestamp.
	_, err := q.Exec(`
        UPDATE boards SET last_activity_at = NOW() AT TIME ZONE 'utc'
        WHERE short_name = $1`,
		board,
	)
	if err != nil {
		return false, fmt.Errorf("failed to update board activity on pin toggle: %w", err)
	}

	// Toggle the thread's pinned status and return the new value.
	var newStatus bool
	err = q.QueryRow(
		"UPDATE threads SET is_pinned = NOT is_pinned WHERE board = $1 AND id = $2 RETURNING is_pinned",
		board, threadId,
	).Scan(&newStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, &internal_errors.ErrorWithStatusCode{Message: "Thread not found", StatusCode: http.StatusNotFound}
		}
		return false, fmt.Errorf("failed to toggle thread pinned status: %w", err)
	}

	return newStatus, nil
}
