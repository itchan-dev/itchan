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
	"github.com/itchan-dev/itchan/shared/logger"
	"github.com/itchan-dev/itchan/shared/utils"
	"github.com/lib/pq"
)

// =========================================================================
// Public Methods (satisfy the service.MessageStorage interface)
// =========================================================================

// CreateMessage serves as the public entry point for creating a new message.
// It is responsible for wrapping the core message creation logic in a single,
// atomic database transaction. This ensures that all related database operations
// (updating board/thread metadata, inserting the message, attachments, and replies)
// either succeed together or fail together, maintaining data integrity.
func (s *Storage) CreateMessage(creationData domain.MessageCreationData, attachments domain.Attachments) (domain.MsgId, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var msgID domain.MsgId
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		var err error
		msgID, err = s.createMessage(tx, creationData)
		if err != nil {
			return err
		}

		// Add attachments in the same transaction
		if len(attachments) > 0 {
			if err := s.addAttachments(tx, creationData.Board, creationData.ThreadId, msgID, attachments); err != nil {
				return err
			}
		}

		return nil
	})
	return msgID, err
}

// DeleteMessage is the public entry point for deleting a message.
// It manages the transaction for this operation, ensuring that the board's
// last activity is updated and the message is deleted atomically. The cascading
// deletion of related attachments and replies is handled by the database schema.
func (s *Storage) DeleteMessage(board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.withTx(ctx, func(tx *sql.Tx) error {
		return s.deleteMessage(tx, board, threadId, id)
	})
}

// GetMessage is a read-only operation that fetches a complete message, including its
// attachments and replies. Since it doesn't modify data, it doesn't need to
// create its own transaction. It can use the main database connection pool (s.db)
// as the Querier, allowing for concurrent reads.
func (s *Storage) GetMessage(board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) (domain.Message, error) {
	// Delegate to the internal method, passing the main DB connection pool.
	return s.getMessage(s.db, board, threadId, id)
}

// =========================================================================
// Internal Methods (Core Database Logic)
// These methods accept a Querier and are transaction-agnostic.
// =========================================================================

// createMessage contains the core logic for inserting a new message and its related data.
// It's unexported and accepts a Querier, allowing it to be run within a transaction
// managed by a public method (like CreateMessage or CreateThread) or in a test.
// Returns the message ID (which is per-thread sequential: 1, 2, 3...).
func (s *Storage) createMessage(q Querier, creationData domain.MessageCreationData) (domain.MsgId, error) {
	// Determine the creation timestamp for all related records in this operation.
	// Use the provided timestamp if available (useful for testing or migrations),
	// otherwise generate the current UTC timestamp rounded to microseconds for consistency.
	var createdAt time.Time
	if creationData.CreatedAt != nil {
		createdAt = *creationData.CreatedAt
	} else {
		createdAt = time.Now().UTC().Round(time.Microsecond)
	}

	// Atomically update the parent board's last_activity timestamp.
	result, err := q.Exec(`
	       	UPDATE boards SET
				last_activity_at = GREATEST(last_activity_at, $1)
	       	WHERE short_name = $2
			`,
		createdAt, creationData.Board,
	)
	if err != nil {
		return -1, fmt.Errorf("failed to update board activity: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return -1, &internal_errors.ErrorWithStatusCode{Message: "Board not found", StatusCode: http.StatusNotFound}
	}

	// Update the parent thread's metadata (reply count and bump timestamp) and get the
	// new message's ID (which equals message_count after increment).
	var msgId int64
	err = q.QueryRow(`
	       UPDATE threads SET
	           message_count = message_count + 1,
	           last_bumped_at = CASE WHEN message_count > $1 THEN last_bumped_at ELSE $2 END
	       WHERE board = $3 AND id = $4
		   RETURNING message_count
		   `,
		s.cfg.Public.BumpLimit, createdAt, creationData.Board, creationData.ThreadId,
	).Scan(&msgId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return -1, &internal_errors.ErrorWithStatusCode{Message: "Thread not found", StatusCode: http.StatusNotFound}
		}
		return -1, fmt.Errorf("failed to update thread: %w", err)
	}

	// Insert the message record into its board-specific partition.
	// The message ID is per-thread sequential (1, 2, 3...) - id=1 is always OP.
	partitionName := PartitionName(creationData.Board, "messages")
	_, err = q.Exec(fmt.Sprintf(`
	       INSERT INTO %s (id, author_id, text, created_at, thread_id, updated_at, board)
	       VALUES ($1, $2, $3, $4, $5, $4, $6)`, partitionName),
		msgId, creationData.Author.Id, creationData.Text, createdAt, creationData.ThreadId, creationData.Board,
	)
	if err != nil {
		return -1, fmt.Errorf("failed to insert message: %w", err)
	}

	// If this message is a reply to others, insert those relationships.
	if creationData.ReplyTo != nil {
		for _, reply := range *creationData.ReplyTo {
			replyPartitionName := PartitionName(creationData.Board, "message_replies")
			_, err := q.Exec(fmt.Sprintf(`
			             INSERT INTO %s (board, sender_message_id, sender_thread_id, receiver_message_id, receiver_thread_id, created_at)
			             VALUES ($1, $2, $3, $4, $5, $6)`, replyPartitionName),
				creationData.Board, msgId, creationData.ThreadId, reply.To, reply.ToThreadId, createdAt,
			)
			if err != nil {
				return -1, fmt.Errorf("failed to insert message reply relationship: %w", err)
			}
		}
	}

	return msgId, nil
}

// deleteMessage contains the core logic for removing a message record and updating
// parent metadata. It is unexported and accepts a Querier.
func (s *Storage) deleteMessage(q Querier, board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) error {
	// Collect file IDs BEFORE cascade delete (while attachments still exist)
	rows, err := q.Query(`
		SELECT DISTINCT file_id FROM attachments
		WHERE board = $1 AND thread_id = $2 AND message_id = $3`,
		board, threadId, id)
	if err != nil {
		return fmt.Errorf("failed to get file IDs: %w", err)
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

	// Update the board's last_activity timestamp to reflect the deletion.
	deletedTs := time.Now().UTC().Round(time.Microsecond)
	result, err := q.Exec(`
	       UPDATE boards SET last_activity_at = GREATEST(last_activity_at, $1)
	       WHERE short_name = $2`,
		deletedTs, board,
	)
	if err != nil {
		return fmt.Errorf("failed to update board activity on delete: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return &internal_errors.ErrorWithStatusCode{Message: "Board not found", StatusCode: http.StatusNotFound}
	}

	// Delete the message from its partition. Foreign key constraints with ON DELETE CASCADE
	// will handle the automatic deletion of related attachments and replies.
	result, err = q.Exec("DELETE FROM messages WHERE board = $1 AND thread_id = $2 AND id = $3", board, threadId, id)
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return &internal_errors.ErrorWithStatusCode{Message: "Message not found", StatusCode: http.StatusNotFound}
	}

	// Decrement the thread's message count to maintain consistency
	_, err = q.Exec(`
		UPDATE threads SET message_count = message_count - 1
		WHERE board = $1 AND id = $2`,
		board, threadId,
	)
	if err != nil {
		return fmt.Errorf("failed to update thread message count: %w", err)
	}

	// Delete file records in batch (attachments are already cascade-deleted)
	// FK constraint will prevent deletion if files are still referenced elsewhere
	// This is best-effort - if it fails, the GC will clean up later
	if len(fileIDs) > 0 {
		_, err = q.Exec(`DELETE FROM files WHERE id = ANY($1)`, pq.Array(fileIDs))
		if err != nil {
			// Log warning but don't fail - GC will clean up orphaned files
			logger.Log.Warn("failed to delete file records during message deletion",
				"board", board,
				"thread_id", threadId,
				"message_id", id,
				"file_count", len(fileIDs),
				"error", err)
		}
	}

	return nil
}

// getMessage contains the core logic for fetching a message and all its related data.
// It composes several helper functions to build the complete domain.Message object.
func (s *Storage) getMessage(q Querier, board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) (domain.Message, error) {
	var msg domain.Message
	err := q.QueryRow(`
	   SELECT m.id, m.author_id, u.email_domain, u.is_admin, m.text, m.created_at, m.thread_id, m.updated_at, m.board
	   FROM messages m
	   JOIN users u ON m.author_id = u.id
	   WHERE m.board = $1 AND m.thread_id = $2 AND m.id = $3`,
		board, threadId, id,
	).Scan(
		&msg.Id, &msg.Author.Id, &msg.Author.EmailDomain, &msg.Author.Admin, &msg.Text, &msg.CreatedAt, &msg.ThreadId,
		&msg.ModifiedAt, &msg.Board,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Message{}, &internal_errors.ErrorWithStatusCode{Message: "Message not found", StatusCode: http.StatusNotFound}
		}
		return domain.Message{}, fmt.Errorf("failed to query message: %w", err)
	}

	// Calculate page from message ID (which is per-thread sequential)
	msg.Page = utils.CalculatePage(int(msg.Id), s.cfg.Public.MessagesPerThreadPage)

	// Fetch and attach related data using helper functions.
	attachments, err := s.getMessageAttachments(q, board, threadId, id)
	if err != nil {
		return domain.Message{}, err
	}
	msg.Attachments = attachments

	replies, err := s.getMessageRepliesTo(q, board, threadId, id) // Replies *to* this message
	if err != nil {
		return domain.Message{}, err
	}
	msg.Replies = replies

	return msg, nil
}

// getMessageAttachments fetches all attachment records associated with a specific message.
func (s *Storage) getMessageAttachments(q Querier, board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) (domain.Attachments, error) {
	rows, err := q.Query(`
        SELECT a.id, a.board, a.thread_id, a.message_id, a.file_id,
               f.file_path, f.filename, f.original_filename, f.file_size_bytes, f.mime_type, f.original_mime_type, f.image_width, f.image_height, f.thumbnail_path
        FROM attachments a
        JOIN files f ON a.file_id = f.id
        WHERE a.board = $1 AND a.thread_id = $2 AND a.message_id = $3
        ORDER BY a.id`,
		board, threadId, id,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch message attachments: %w", err)
	}
	defer rows.Close()

	var attachments domain.Attachments
	for rows.Next() {
		var attachment domain.Attachment
		var file domain.File
		if err := rows.Scan(
			&attachment.Id, &attachment.Board, &attachment.ThreadId, &attachment.MessageId, &attachment.FileId,
			&file.FilePath, &file.Filename, &file.OriginalFilename, &file.SizeBytes, &file.MimeType, &file.OriginalMimeType, &file.ImageWidth, &file.ImageHeight, &file.ThumbnailPath,
		); err != nil {
			return nil, fmt.Errorf("failed to scan attachment row: %w", err)
		}
		attachment.File = &file
		attachments = append(attachments, &attachment)
	}
	return attachments, rows.Err()
}

// getMessageRepliesTo fetches all reply relationships where the specified message is the *receiver*.
func (s *Storage) getMessageRepliesTo(q Querier, board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) (domain.Replies, error) {
	rows, err := q.Query(`
	       SELECT mr.board, mr.sender_message_id, mr.sender_thread_id, mr.receiver_message_id, mr.receiver_thread_id, mr.created_at
	       FROM message_replies mr
	       WHERE mr.board = $1 AND mr.receiver_thread_id = $2 AND mr.receiver_message_id = $3
	       ORDER BY mr.created_at`,
		board, threadId, id,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch message replies: %w", err)
	}
	defer rows.Close()

	var replies domain.Replies
	for rows.Next() {
		var reply domain.Reply
		if err := rows.Scan(&reply.Board, &reply.From, &reply.FromThreadId, &reply.To, &reply.ToThreadId, &reply.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan reply row: %w", err)
		}
		// From (sender_message_id) is now the per-thread sequential ID, which is also the ordinal
		reply.FromPage = utils.CalculatePage(int(reply.From), s.cfg.Public.MessagesPerThreadPage)
		replies = append(replies, &reply)
	}
	return replies, rows.Err()
}

// getMessageRepliesFrom fetches all reply relationships where the specified message is the *sender*.
func (s *Storage) getMessageRepliesFrom(q Querier, board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) (domain.Replies, error) {
	rows, err := q.Query(`
	       SELECT board, sender_message_id, sender_thread_id, receiver_message_id, receiver_thread_id, created_at
	       FROM message_replies
	       WHERE board = $1 AND sender_thread_id = $2 AND sender_message_id = $3
	       ORDER BY created_at`,
		board, threadId, id,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch message replies from: %w", err)
	}
	defer rows.Close()

	var replies domain.Replies
	for rows.Next() {
		var reply domain.Reply
		if err := rows.Scan(&reply.Board, &reply.From, &reply.FromThreadId, &reply.To, &reply.ToThreadId, &reply.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan reply row from: %w", err)
		}
		replies = append(replies, &reply)
	}
	return replies, rows.Err()
}

// addAttachments is the internal method to add attachments within a transaction
func (s *Storage) addAttachments(q Querier, board domain.BoardShortName, threadId domain.ThreadId, messageID domain.MsgId, attachments domain.Attachments) error {
	for _, attachment := range attachments {
		// Insert file record
		var fileId int64
		err := q.QueryRow(`
            INSERT INTO files (file_path, filename, original_filename, file_size_bytes, mime_type, original_mime_type, image_width, image_height, thumbnail_path)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
			attachment.File.FilePath, attachment.File.Filename, attachment.File.OriginalFilename, attachment.File.SizeBytes,
			attachment.File.MimeType, attachment.File.OriginalMimeType, attachment.File.ImageWidth, attachment.File.ImageHeight, attachment.File.ThumbnailPath,
		).Scan(&fileId)
		if err != nil {
			return fmt.Errorf("failed to insert file: %w", err)
		}

		// Insert attachment record
		attachPartitionName := PartitionName(board, "attachments")
		_, err = q.Exec(fmt.Sprintf(`
            INSERT INTO %s (board, thread_id, message_id, file_id) VALUES ($1, $2, $3, $4)`, attachPartitionName),
			board, threadId, messageID, fileId,
		)
		if err != nil {
			return fmt.Errorf("failed to insert attachment link: %w", err)
		}
	}

	return nil
}
