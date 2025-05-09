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

func (s *Storage) CreateMessage(creationData domain.MessageCreationData, isOp bool, outerTx *sql.Tx) (domain.MsgId, error) {
	var tx *sql.Tx
	var err error
	if outerTx != nil {
		tx = outerTx
	} else {
		tx, err = s.db.Begin()
		if err != nil {
			return -1, fmt.Errorf("failed to begin transaction: %w", err)
		}
		defer tx.Rollback()
	}

	// Handle created timestamp
	var createdTs time.Time
	if creationData.CreatedAt != nil {
		createdTs = *creationData.CreatedAt
	} else {
		createdTs = time.Now().UTC().Round(time.Microsecond)
	}

	// Update board's last_activity
	result, err := tx.Exec(`
        UPDATE boards 
        SET last_activity = CASE WHEN $1 > last_activity THEN $1 ELSE last_activity END
        WHERE short_name = $2`,
		createdTs, creationData.Board,
	)
	if err != nil {
		return -1, fmt.Errorf("failed to update board activity: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return -1, &internal_errors.ErrorWithStatusCode{
			Message:    "Board not found",
			StatusCode: http.StatusNotFound,
		}
	}

	// Update thread metadata:
	// - Bump num_replies unless it's an OP message
	// - Update last_bump_ts only if under bump limit
	var ordinal int64
	err = tx.QueryRow(`
        UPDATE threads 
        SET 
            num_replies = CASE WHEN $1 THEN num_replies ELSE num_replies + 1 END,
            last_bump_ts = CASE WHEN num_replies >= $2 THEN last_bump_ts ELSE $3 END 
        WHERE board = $4 AND id = $5 
        RETURNING num_replies`,
		isOp,
		s.cfg.Public.BumpLimit,
		createdTs,
		creationData.Board,
		creationData.ThreadId,
	).Scan(&ordinal)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return -1, &internal_errors.ErrorWithStatusCode{
				Message:    "Thread not found",
				StatusCode: http.StatusNotFound,
			}
		}
		return -1, fmt.Errorf("failed to update thread: %w", err)
	}

	// Insert message into partition
	var id int64
	partitionName := MessagesPartitionName(creationData.Board)
	err = tx.QueryRow(fmt.Sprintf(`
        INSERT INTO %s (
            author_id, text, created, attachments, 
            thread_id, ordinal, modified, op, board
        ) 
        VALUES ($1, $2, $3, $4, $5, $6, $3, $7, $8) 
        RETURNING id`, partitionName),
		creationData.Author.Id,
		creationData.Text,
		createdTs,
		creationData.Attachments,
		creationData.ThreadId,
		ordinal,
		isOp,
		creationData.Board,
	).Scan(&id)
	if err != nil {
		return -1, fmt.Errorf("failed to insert message: %w", err)
	}

	if outerTx == nil {
		if err := tx.Commit(); err != nil {
			return -1, fmt.Errorf("failed to commit transaction: %w", err)
		}
	}
	return id, nil
}

func (s *Storage) GetMessage(board domain.BoardShortName, id domain.MsgId) (domain.Message, error) {
	var msg domain.Message
	err := s.db.QueryRow(`
	SELECT
		*
	FROM messages
	WHERE
	board = $1
	AND id = $2`, board, id).Scan(
		&msg.Id, &msg.Author.Id, &msg.Text, &msg.CreatedAt, &msg.Attachments, &msg.ThreadId, &msg.Ordinal, &msg.ModifiedAt, &msg.Op, &msg.Board,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Message{}, &internal_errors.ErrorWithStatusCode{Message: "Message not found", StatusCode: http.StatusNotFound}
		}
		return domain.Message{}, err
	}
	return msg, nil
}

func (s *Storage) DeleteMessage(board domain.BoardShortName, id domain.MsgId) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// step 1: update board last_activity
	deletedTs := time.Now().UTC().Round(time.Microsecond) // database anyway round to microsecond
	result, err := tx.Exec(`
	UPDATE boards SET
		last_activity = CASE WHEN $2 > last_activity THEN $2 ELSE last_activity END
	WHERE short_name = $1
	`,
		board, deletedTs)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return &internal_errors.ErrorWithStatusCode{Message: "Board not found", StatusCode: http.StatusNotFound}
	}

	// step 2: delete from messages (change text and remove attachments)
	result, err = tx.Exec(`
	UPDATE messages SET
		text = 'deleted',
		attachments = NULL,
		modified = $1
	WHERE
	board = $2
	AND id = $3
	`,
		deletedTs, board, id)
	if err != nil {
		return err
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if deleted == 0 {
		return &internal_errors.ErrorWithStatusCode{Message: "Message not found", StatusCode: http.StatusNotFound}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}
