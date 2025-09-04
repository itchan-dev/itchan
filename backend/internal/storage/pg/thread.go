package pg

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
	internal_errors "github.com/itchan-dev/itchan/shared/errors"
)

func (s *Storage) CreateThread(creationData domain.ThreadCreationData) (domain.ThreadId, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return -1, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Verify board exists
	var board domain.BoardShortName
	err = tx.QueryRow(
		"SELECT short_name FROM boards WHERE short_name = $1",
		creationData.Board,
	).Scan(&board)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return -1, &internal_errors.ErrorWithStatusCode{
				Message:    "Board not found",
				StatusCode: http.StatusNotFound,
			}
		}
		return -1, fmt.Errorf("failed to validate board: %w", err)
	}

	// Insert into thread partition
	var id domain.ThreadId
	var createdTs time.Time
	partitionName := ThreadsPartitionName(creationData.Board)
	err = tx.QueryRow(
		fmt.Sprintf(`
            INSERT INTO %s (title, board, is_sticky) 
            VALUES ($1, $2, $3) 
            RETURNING id, last_bump_ts
        `, partitionName),
		creationData.Title,
		creationData.Board,
		creationData.IsSticky,
	).Scan(&id, &createdTs)
	if err != nil {
		return -1, fmt.Errorf("failed to insert thread: %w", err)
	}

	// Create OP message within the same transaction
	creationData.OpMessage.ThreadId = id
	creationData.OpMessage.CreatedAt = &createdTs
	creationData.OpMessage.Board = creationData.Board
	if _, err = s.CreateMessage(creationData.OpMessage, true, tx); err != nil {
		return -1, fmt.Errorf("failed to create OP message: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return -1, fmt.Errorf("failed to commit transaction: %w", err)
	}
	return id, nil
}

func (s *Storage) GetThread(board domain.BoardShortName, id domain.ThreadId) (domain.Thread, error) {
	var metadata domain.ThreadMetadata
	err := s.db.QueryRow(`
        SELECT 
            id, title, board, num_replies, last_bump_ts, is_sticky 
        FROM threads 
        WHERE board = $1 AND id = $2
    `, board, id).Scan(
		&metadata.Id, &metadata.Title, &metadata.Board,
		&metadata.NumReplies, &metadata.LastBumped, &metadata.IsSticky,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Thread{}, &internal_errors.ErrorWithStatusCode{
				Message:    "Thread not found",
				StatusCode: http.StatusNotFound,
			}
		}
		return domain.Thread{}, fmt.Errorf("failed to fetch thread metadata: %w", err)
	}

	// Fetch all messages for this thread
	rows, err := s.db.Query(`
        SELECT 
            id, author_id, text, created, attachments, 
            thread_id, ordinal, modified, op, board 
        FROM messages 
        WHERE board = $1 AND thread_id = $2 
        ORDER BY created
    `, board, id)
	if err != nil {
		return domain.Thread{}, fmt.Errorf("failed to fetch messages: %w", err)
	}
	defer rows.Close()

	var messages []*domain.Message
	msgIdxMap := make(map[int64]int)
	for rows.Next() {
		var msg domain.Message
		if err := rows.Scan(
			&msg.Id, &msg.Author.Id, &msg.Text, &msg.CreatedAt,
			&msg.Attachments, &msg.ThreadId, &msg.Ordinal,
			&msg.ModifiedAt, &msg.Op, &msg.Board,
		); err != nil {
			return domain.Thread{}, fmt.Errorf("failed to scan message: %w", err)
		}
		messages = append(messages, &msg)
		msgIdxMap[msg.Id] = len(messages) - 1
	}
	if err = rows.Err(); err != nil {
		return domain.Thread{}, fmt.Errorf("rows iteration error: %w", err)
	}

	// Fetch all replies for this thread
	replyRows, err := s.db.Query(`
        SELECT sender_message_id, sender_thread_id, receiver_message_id, receiver_thread_id, created
        FROM message_replies
        WHERE board = $1 AND receiver_thread_id = $2
        ORDER BY created
    `, board, id)
	if err != nil {
		return domain.Thread{}, fmt.Errorf("failed to fetch message replies: %w", err)
	}
	defer replyRows.Close()
	for replyRows.Next() {
		var reply domain.Reply
		if err := replyRows.Scan(&reply.From, &reply.FromThreadId, &reply.To, &reply.ToThreadId, &reply.CreatedAt); err != nil {
			return domain.Thread{}, fmt.Errorf("failed to scan reply row: %w", err)
		}
		if msgIdx, ok := msgIdxMap[reply.To]; ok {
			msg := messages[msgIdx]
			msg.Replies = append(msg.Replies, &reply)
		}
	}

	return domain.Thread{ThreadMetadata: metadata, Messages: messages}, nil
}

func (s *Storage) DeleteThread(board domain.BoardShortName, id domain.MsgId) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Update board's last_activity to now
	result, err := tx.Exec(`
        UPDATE boards 
        SET last_activity = NOW() AT TIME ZONE 'utc' 
        WHERE short_name = $1
    `, board)
	if err != nil {
		return fmt.Errorf("failed to update board activity: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return &internal_errors.ErrorWithStatusCode{
			Message:    "Board not found",
			StatusCode: http.StatusNotFound,
		}
	}

	// Delete thread (messages cascade via foreign key)
	result, err = tx.Exec(
		"DELETE FROM threads WHERE board = $1 AND id = $2",
		board, id,
	)
	if err != nil {
		return fmt.Errorf("failed to delete thread: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return &internal_errors.ErrorWithStatusCode{
			Message:    "Thread not found",
			StatusCode: http.StatusNotFound,
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}
