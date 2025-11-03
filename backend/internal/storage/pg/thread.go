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
// messages, replies, and attachments. As a read-only operation, it does not
// need to manage a transaction and can delegate directly to the internal method
// using the main database connection pool for efficiency.
func (s *Storage) GetThread(board domain.BoardShortName, id domain.ThreadId) (domain.Thread, error) {
	return s.getThread(s.db, board, id)
}

// DeleteThread is the public entry point for deleting a thread. It wraps the core
// deletion logic in a transaction to ensure atomicity. The database schema's
// foreign key constraints will cascade the delete from the thread to all of its
// contained messages, attachments, and replies.
func (s *Storage) DeleteThread(board domain.BoardShortName, id domain.MsgId) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.withTx(ctx, func(tx *sql.Tx) error {
		return s.deleteThread(tx, board, id)
	})
}

// =========================================================================
// Internal Methods (Core Database Logic)
// These methods accept a Querier and are transaction-agnostic.
// =========================================================================

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
	           INSERT INTO %s (title, board, is_sticky, created_at)
	           VALUES ($1, $2, $3, $4)
	           RETURNING id, created_at
	       `, partitionName),
		creationData.Title,
		creationData.Board,
		creationData.IsSticky,
		createdAt,
	).Scan(&id, &createdTs)
	if err != nil {
		return -1, time.Time{}, fmt.Errorf("failed to insert thread record: %w", err)
	}

	return id, createdTs, nil
}

// getThread contains the comprehensive logic for fetching all data related to a
// single thread. It queries multiple tables and assembles the data into a
// cohesive `domain.Thread` object. It accepts a Querier to be testable and
// usable within or outside a transaction.
func (s *Storage) getThread(q Querier, board domain.BoardShortName, id domain.ThreadId) (domain.Thread, error) {
	var metadata domain.ThreadMetadata
	err := q.QueryRow(`
	       	SELECT
				id, title, board, message_count, last_bumped_at, is_sticky
	       	FROM threads
		   	WHERE board = $1 AND id = $2`,
		board, id,
	).Scan(
		&metadata.Id, &metadata.Title, &metadata.Board,
		&metadata.MessageCount, &metadata.LastBumped, &metadata.IsSticky,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Thread{}, &internal_errors.ErrorWithStatusCode{Message: "Thread not found", StatusCode: http.StatusNotFound}
		}
		return domain.Thread{}, fmt.Errorf("failed to fetch thread metadata: %w", err)
	}

	// Fetch all messages for the thread.
	msgRows, err := q.Query(`
		SELECT
			id, author_id, text, created_at, thread_id, ordinal, updated_at, is_op, board
		FROM messages
		WHERE board = $1 AND thread_id = $2
		ORDER BY created_at`,
		board, id,
	)
	if err != nil {
		return domain.Thread{}, fmt.Errorf("failed to fetch thread messages: %w", err)
	}
	defer msgRows.Close()

	var messages []*domain.Message
	msgIDMap := make(map[domain.MsgId]*domain.Message)
	for msgRows.Next() {
		var msg domain.Message
		if err := msgRows.Scan(
			&msg.Id, &msg.Author.Id, &msg.Text, &msg.CreatedAt,
			&msg.ThreadId, &msg.Ordinal, &msg.ModifiedAt, &msg.Op, &msg.Board,
		); err != nil {
			return domain.Thread{}, fmt.Errorf("failed to scan message row: %w", err)
		}
		messages = append(messages, &msg)
		msgIDMap[msg.Id] = &msg // Store a pointer to the message for easy lookup.
	}
	if err = msgRows.Err(); err != nil {
		return domain.Thread{}, fmt.Errorf("error iterating message rows: %w", err)
	}

	// Fetch all reply relationships for the entire thread in a single query for efficiency.
	replyRows, err := q.Query(`
        SELECT
			board, sender_message_id, sender_thread_id, receiver_message_id, receiver_thread_id, created_at
        FROM message_replies
		WHERE board = $1 AND receiver_thread_id = $2
		ORDER BY created_at`,
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
		// Attach the reply to the correct message using the map.
		if msg, ok := msgIDMap[reply.To]; ok {
			msg.Replies = append(msg.Replies, &reply)
		}
	}

	// Fetch all attachments for the entire thread in a single query.
	attachRows, err := q.Query(`
        SELECT
			a.id, a.board, a.message_id, a.file_id,
            f.file_path, f.original_filename, f.file_size_bytes, f.mime_type, f.image_width, f.image_height
        FROM attachments a
        JOIN files f ON a.file_id = f.id
        JOIN messages m ON a.message_id = m.id AND a.board = m.board
        WHERE a.board = $1 AND m.thread_id = $2
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
			&attachment.Id, &attachment.Board, &attachment.MessageId, &attachment.FileId,
			&file.FilePath, &file.OriginalFilename, &file.FileSizeBytes, &file.MimeType, &file.ImageWidth, &file.ImageHeight,
		); err != nil {
			return domain.Thread{}, fmt.Errorf("failed to scan attachment row: %w", err)
		}
		attachment.File = &file
		// Attach the attachment to the correct message using the map.
		if msg, ok := msgIDMap[attachment.MessageId]; ok {
			msg.Attachments = append(msg.Attachments, &attachment)
		}
	}

	return domain.Thread{ThreadMetadata: metadata, Messages: messages}, nil
}

// deleteThread contains the core logic for removing a thread record.
func (s *Storage) deleteThread(q Querier, board domain.BoardShortName, id domain.MsgId) error {
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
