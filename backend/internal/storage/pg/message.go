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
// Public Methods (satisfy the service.MessageStorage interface)
// =========================================================================

// CreateMessage serves as the public entry point for creating a new message.
// It is responsible for wrapping the core message creation logic in a single,
// atomic database transaction. This ensures that all related database operations
// (updating board/thread metadata, inserting the message, attachments, and replies)
// either succeed together or fail together, maintaining data integrity.
func (s *Storage) CreateMessage(creationData domain.MessageCreationData) (domain.MsgId, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var msgID domain.MsgId
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		var err error
		msgID, err = s.createMessage(tx, creationData)
		return err
	})
	return msgID, err
}

// DeleteMessage is the public entry point for deleting a message.
// It manages the transaction for this operation, ensuring that the board's
// last activity is updated and the message is deleted atomically. The cascading
// deletion of related attachments and replies is handled by the database schema.
func (s *Storage) DeleteMessage(board domain.BoardShortName, id domain.MsgId) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.withTx(ctx, func(tx *sql.Tx) error {
		return s.deleteMessage(tx, board, id)
	})
}

// GetMessage is a read-only operation that fetches a complete message, including its
// attachments and replies. Since it doesn't modify data, it doesn't need to
// create its own transaction. It can use the main database connection pool (s.db)
// as the Querier, allowing for concurrent reads.
func (s *Storage) GetMessage(board domain.BoardShortName, id domain.MsgId) (domain.Message, error) {
	// Delegate to the internal method, passing the main DB connection pool.
	return s.getMessage(s.db, board, id)
}

// =========================================================================
// Internal Methods (Core Database Logic)
// These methods accept a Querier and are transaction-agnostic.
// =========================================================================

// createMessage contains the core logic for inserting a new message and its related data.
// It's unexported and accepts a Querier, allowing it to be run within a transaction
// managed by a public method (like CreateMessage or CreateThread) or in a test.
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
	// new message's ordinal number in return.
	var ordinal int64
	err = q.QueryRow(`
	       UPDATE threads SET
	           message_count = message_count + 1,
	           last_bumped_at = CASE WHEN message_count > $1 THEN last_bumped_at ELSE $2 END
	       WHERE board = $3 AND id = $4
		   RETURNING message_count
		   `,
		s.cfg.Public.BumpLimit, createdAt, creationData.Board, creationData.ThreadId,
	).Scan(&ordinal)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return -1, &internal_errors.ErrorWithStatusCode{Message: "Thread not found", StatusCode: http.StatusNotFound}
		}
		return -1, fmt.Errorf("failed to update thread: %w", err)
	}

	// Insert the message record into its board-specific partition.
	var id int64
	partitionName := PartitionName(creationData.Board, "messages")
	isOp := ordinal == 1
	err = q.QueryRow(fmt.Sprintf(`
	       INSERT INTO %s (author_id, text, created_at, thread_id, ordinal, updated_at, is_op, board)
	       VALUES ($1, $2, $3, $4, $5, $3, $6, $7) RETURNING id`, partitionName),
		creationData.Author.Id, creationData.Text, createdAt, creationData.ThreadId, ordinal, isOp, creationData.Board,
	).Scan(&id)
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
				creationData.Board, id, creationData.ThreadId, reply.To, reply.ToThreadId, createdAt,
			)
			if err != nil {
				return -1, fmt.Errorf("failed to insert message reply relationship: %w", err)
			}
		}
	}

	return id, nil
}

// deleteMessage contains the core logic for removing a message record and updating
// parent metadata. It is unexported and accepts a Querier.
func (s *Storage) deleteMessage(q Querier, board domain.BoardShortName, id domain.MsgId) error {
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
	result, err = q.Exec("DELETE FROM messages WHERE board = $1 AND id = $2", board, id)
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return &internal_errors.ErrorWithStatusCode{Message: "Message not found", StatusCode: http.StatusNotFound}
	}

	return nil
}

// getMessage contains the core logic for fetching a message and all its related data.
// It composes several helper functions to build the complete domain.Message object.
func (s *Storage) getMessage(q Querier, board domain.BoardShortName, id domain.MsgId) (domain.Message, error) {
	var msg domain.Message
	err := q.QueryRow(`
	   SELECT id, author_id, text, created_at, thread_id, ordinal, updated_at, is_op, board
	   FROM messages 
	   WHERE board = $1 AND id = $2`,
		board, id,
	).Scan(
		&msg.Id, &msg.Author.Id, &msg.Text, &msg.CreatedAt, &msg.ThreadId,
		&msg.Ordinal, &msg.ModifiedAt, &msg.Op, &msg.Board,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Message{}, &internal_errors.ErrorWithStatusCode{Message: "Message not found", StatusCode: http.StatusNotFound}
		}
		return domain.Message{}, fmt.Errorf("failed to query message: %w", err)
	}

	// Fetch and attach related data using helper functions.
	attachments, err := s.getMessageAttachments(q, board, id)
	if err != nil {
		return domain.Message{}, err
	}
	msg.Attachments = attachments

	replies, err := s.getMessageRepliesTo(q, board, id) // Replies *to* this message
	if err != nil {
		return domain.Message{}, err
	}
	msg.Replies = replies

	return msg, nil
}

// getMessageAttachments fetches all attachment records associated with a specific message.
func (s *Storage) getMessageAttachments(q Querier, board domain.BoardShortName, id domain.MsgId) (domain.Attachments, error) {
	rows, err := q.Query(`
        SELECT a.id, a.board, a.message_id, a.file_id,
               f.file_path, f.original_filename, f.file_size_bytes, f.mime_type, f.original_mime_type, f.image_width, f.image_height, f.thumbnail_path
        FROM attachments a
        JOIN files f ON a.file_id = f.id
        WHERE a.board = $1 AND a.message_id = $2
        ORDER BY a.id`,
		board, id,
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
			&attachment.Id, &attachment.Board, &attachment.MessageId, &attachment.FileId,
			&file.FilePath, &file.OriginalFilename, &file.FileSizeBytes, &file.MimeType, &file.OriginalMimeType, &file.ImageWidth, &file.ImageHeight, &file.ThumbnailPath,
		); err != nil {
			return nil, fmt.Errorf("failed to scan attachment row: %w", err)
		}
		attachment.File = &file
		attachments = append(attachments, &attachment)
	}
	return attachments, rows.Err()
}

// getMessageRepliesTo fetches all reply relationships where the specified message is the *receiver*.
func (s *Storage) getMessageRepliesTo(q Querier, board domain.BoardShortName, id domain.MsgId) (domain.Replies, error) {
	rows, err := q.Query(`
	       SELECT board, sender_message_id, sender_thread_id, receiver_message_id, receiver_thread_id, created_at
	       FROM message_replies
	       WHERE board = $1 AND receiver_message_id = $2
	       ORDER BY created_at`,
		board, id,
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
		replies = append(replies, &reply)
	}
	return replies, rows.Err()
}

// getMessageRepliesFrom fetches all reply relationships where the specified message is the *sender*.
func (s *Storage) getMessageRepliesFrom(q Querier, board domain.BoardShortName, id domain.MsgId) (domain.Replies, error) {
	rows, err := q.Query(`
	       SELECT board, sender_message_id, sender_thread_id, receiver_message_id, receiver_thread_id, created_at
	       FROM message_replies
	       WHERE board = $1 AND sender_message_id = $2
	       ORDER BY created_at`,
		board, id,
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

// AddAttachments adds attachments to an existing message.
// This is used when files are uploaded and saved after the message is created.
func (s *Storage) AddAttachments(board domain.BoardShortName, messageID domain.MsgId, attachments domain.Attachments) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.withTx(ctx, func(tx *sql.Tx) error {
		return s.addAttachments(tx, board, messageID, attachments)
	})
}

// addAttachments is the internal method to add attachments within a transaction
func (s *Storage) addAttachments(q Querier, board domain.BoardShortName, messageID domain.MsgId, attachments domain.Attachments) error {
	for _, attachment := range attachments {
		// Insert file record
		var fileId int64
		err := q.QueryRow(`
            INSERT INTO files (file_path, original_filename, file_size_bytes, mime_type, original_mime_type, image_width, image_height, thumbnail_path)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
			attachment.File.FilePath, attachment.File.OriginalFilename, attachment.File.FileSizeBytes,
			attachment.File.MimeType, attachment.File.OriginalMimeType, attachment.File.ImageWidth, attachment.File.ImageHeight, attachment.File.ThumbnailPath,
		).Scan(&fileId)
		if err != nil {
			return fmt.Errorf("failed to insert file: %w", err)
		}

		// Insert attachment record
		attachPartitionName := PartitionName(board, "attachments")
		_, err = q.Exec(fmt.Sprintf(`
            INSERT INTO %s (board, message_id, file_id) VALUES ($1, $2, $3)`, attachPartitionName),
			board, messageID, fileId,
		)
		if err != nil {
			return fmt.Errorf("failed to insert attachment link: %w", err)
		}
	}

	return nil
}
