package pg

import (
	"fmt"

	"github.com/itchan-dev/itchan/shared/domain"
	_ "github.com/lib/pq"
)

// transactional
func (s *Storage) CreateThread(title, board string, msg *domain.Message) (*domain.Thread, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() // The rollback will be ignored if the tx has been committed later in the function.

	var id, createdTs int64
	err = tx.QueryRow("INSERT INTO messages(author_id, text, attachments) VALUES($1, $2, $3) RETURNING id, created", msg.Author.Id, msg.Text, msg.Attachments).Scan(&id, &createdTs)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec("INSERT INTO threads(id, title, board) VALUES($1, $2, $3)", id, title, board)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec("INSERT INTO thread_reply_counter(id, last_reply_ts) VALUES($1, $2)", id, createdTs)
	if err != nil {
		return nil, err
	}

	view_name := get_view_name(board)
	_, err = tx.Exec("REFRESH MATERIALIZED VIEW CONCURRENTLY $1", view_name)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &domain.Thread{Title: title, Messages: []*domain.Message{msg}, Board: board, NumReplies: 0}, nil
}

func (s *Storage) GetThread(id int64) (*domain.Thread, error) {
	var metadata struct {
		title      string
		board      string
		numReplies uint
	}

	err := s.db.QueryRow(`
    SELECT 
        title,
        board,
        NumReplies
    FROM threads as t 
    LEFT JOIN thread_reply_counter as trc 
        ON t.id = trc.id
    WHERE t.id = $1
    `, id).Scan(&metadata.title, &metadata.board, &metadata.numReplies)
	if err != nil {
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

	return &domain.Thread{Title: metadata.title, Messages: messages, Board: metadata.board, NumReplies: metadata.numReplies}, nil
}

// cascade delete
func (s *Storage) DeleteThread(board string, id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // The rollback will be ignored if the tx has been committed later in the function.

	_, err = tx.Exec("DELETE FROM messages WHERE COALESCE(thread_id, id) = $1", id)
	if err != nil {
		return err
	}

	view_name := get_view_name(board)
	_, err = tx.Exec("REFRESH MATERIALIZED VIEW CONCURRENTLY $1", view_name)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}
