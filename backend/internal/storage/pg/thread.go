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

// transactional
func (s *Storage) CreateThread(title, board string, msg *domain.Message) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return -1, err
	}
	defer tx.Rollback() // The rollback will be ignored if the tx has been committed later in the function.

	createdTs := time.Now().UTC().Round(time.Microsecond) // database anyway round to microsecond
	result, err := tx.Exec(`
	UPDATE boards SET
		last_activity = $2
	WHERE short_name = $1
	`,
		board, createdTs)
	if err != nil {
		return -1, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return -1, err
	}
	if affected == 0 {
		return -1, &internal_errors.ErrorWithStatusCode{Message: "Board not found", StatusCode: 404}
	}

	var id int64
	err = tx.QueryRow("INSERT INTO messages(author_id, text, attachments, created, modified) VALUES($1, $2, $3, $4, $5) RETURNING id", msg.Author.Id, msg.Text, msg.Attachments, createdTs, createdTs).Scan(&id)
	if err != nil { // catch unique violation error and raise "user already exists"
		return -1, err
	}

	_, err = tx.Exec("INSERT INTO threads(id, title, board, last_bump_ts) VALUES($1, $2, $3, $4)", id, title, board, createdTs)
	if err != nil {
		return -1, err
	}

	if err := tx.Commit(); err != nil {
		return -1, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return id, nil
}

func (s *Storage) GetThread(id int64) (*domain.Thread, error) {
	var metadata struct {
		title      string
		board      string
		numReplies int
		lastBumped time.Time
	}

	err := s.db.QueryRow(`
    SELECT 
        title,
        board,
        reply_count,
		last_bump_ts
    FROM threads
    WHERE id = $1
    `, id).Scan(&metadata.title, &metadata.board, &metadata.numReplies, &metadata.lastBumped)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &internal_errors.ErrorWithStatusCode{Message: "Thread not found", StatusCode: 404}
		}
		return nil, err
	}

	rows, err := s.db.Query(`
	SELECT 
		id
        ,author_id
        ,text
        ,created
        ,attachments
        ,thread_id
	FROM messages 
	WHERE COALESCE(thread_id, id) = $1
    ORDER BY created
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*domain.Message
	for rows.Next() {
		var msg domain.Message
		err = rows.Scan(&msg.Id, &msg.Author.Id, &msg.Text, &msg.CreatedAt, &msg.Attachments, &msg.ThreadId)
		if err != nil {
			return nil, err
		}
		messages = append(messages, &msg)
	}
	if rows.Err() != nil {
		return nil, err
	}

	return &domain.Thread{Title: metadata.title, Messages: messages, Board: metadata.board, NumReplies: metadata.numReplies, LastBumped: metadata.lastBumped}, nil
}

// cascade delete
func (s *Storage) DeleteThread(board string, id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // The rollback will be ignored if the tx has been committed later in the function.

	result, err := tx.Exec(`
	UPDATE boards SET
		last_activity = (now() at time zone 'utc')
	WHERE short_name = $1
	`,
		board)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return &internal_errors.ErrorWithStatusCode{Message: "Board not found", StatusCode: 404}
	}

	result, err = tx.Exec("DELETE FROM messages WHERE COALESCE(thread_id, id) = $1", id)
	if err != nil {
		return err
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if deleted == 0 {
		return &internal_errors.ErrorWithStatusCode{Message: "Thread not found", StatusCode: 404}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func (s *Storage) ThreadCount(board string) (int, error) {
	var count int
	err := s.db.QueryRow(`
	SELECT 
		count(*) as count
	FROM threads
	WHERE board = $1
	`, board).Scan(&count)
	if err != nil {
		return -1, err
	}
	return count, nil
}

func (s *Storage) LastThreadId(board string) (int64, error) {
	var id int64
	err := s.db.QueryRow(`
	SELECT 
		id
	FROM threads
	WHERE board = $1 and is_sticky = FALSE
	ORDER BY last_bump_ts
	LIMIT 1
	`, board).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return -1, &internal_errors.ErrorWithStatusCode{Message: "There are no threads", StatusCode: 404}
		}
		return -1, err
	}
	return id, nil
}
