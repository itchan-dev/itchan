package pg

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
	internal_errors "github.com/itchan-dev/itchan/shared/errors"

	_ "github.com/lib/pq"
)

// Saves message to db
func (s *Storage) CreateMessage(board string, author *domain.User, text string, attachments *domain.Attachments, threadId int64) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return -1, err
	}
	defer tx.Rollback() // The rollback will be ignored if the tx has been committed later in the function.

	var n int64
	createdTs := time.Now().UTC().Round(time.Microsecond) // database anyway round to microsecond
	err = tx.QueryRow(`
	UPDATE threads
	SET reply_count = reply_count + 1, last_bump_ts = CASE WHEN reply_count > $1 THEN last_bump_ts ELSE $2 END -- if reply_count over bump limit then dont update last_bump_ts
	WHERE id = $3
	RETURNING reply_count
	`, s.cfg.Public.BumpLimit, createdTs, threadId).Scan(&n)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return -1, &internal_errors.ErrorWithStatusCode{Message: "Thread not found", StatusCode: 404}
		}
		return -1, err
	}

	var id int64
	err = tx.QueryRow(`
	INSERT INTO messages(author_id, text, created, modified, attachments, thread_id, n) 
	VALUES($1, $2, $3, $4, $5, $6, $7) 
	RETURNING id`,
		author.Id, text, createdTs, createdTs, attachments, threadId, n).Scan(&id)
	if err != nil {
		return -1, err
	}

	if err := tx.Commit(); err != nil {
		return -1, err
	}

	return id, nil
}

func (s *Storage) GetMessage(id int64) (*domain.Message, error) {
	var msg domain.Message
	err := s.db.QueryRow(`
	SELECT
		id,
		author_id,
		text,
		created,
		modified,
		attachments,
		thread_id
	FROM messages
	WHERE id = $1`, id).Scan(&msg.Id, &msg.Author.Id, &msg.Text, &msg.CreatedAt, &msg.ModifiedAt, &msg.Attachments, &msg.ThreadId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &internal_errors.ErrorWithStatusCode{Message: "Message not found", StatusCode: 404}
		}
		return nil, err
	}
	return &msg, nil
}

// Saves message to db
func (s *Storage) DeleteMessage(board string, id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // The rollback will be ignored if the tx has been committed later in the function.

	result, err := tx.Exec(`
	UPDATE messages SET
		text = 'msg deleted',
		attachments = NULL,
		modified = (now() at time zone 'utc')
	WHERE id = $1`, id)
	if err != nil {
		return err
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if deleted == 0 {
		return &internal_errors.ErrorWithStatusCode{Message: "Message not found", StatusCode: 404}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}
