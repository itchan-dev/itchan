package pg

import (
	"github.com/itchan-dev/itchan/internal/domain"

	_ "github.com/lib/pq"
)

// Saves message to db
func (s *Storage) CreateMessage(board string, author *domain.User, text string, attachments []domain.Attachment, thread_id int64) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() // The rollback will be ignored if the tx has been committed later in the function.

	var n, createdTs int64
	err = tx.QueryRow(`
	UPDATE thread_reply_counter 
	SET n = n + 1, last_reply_ts = now() 
	WHERE id = $1
	RETURNING n, last_reply_ts
	`, thread_id).Scan(&n, &createdTs)
	if err != nil {
		return 0, err
	}

	var id int64
	err = tx.QueryRow(`
	INSERT INTO messages(author_id, text, created, attachments, thread_id, n) 
	VALUES($1, $2, $3, $4, $5, $6) 
	RETURNING id`,
		author.Id, text, createdTs, attachments, thread_id, n).Scan(&id)
	if err != nil {
		return 0, err
	}

	view_name := get_view_name(board)
	_, err = tx.Exec("REFRESH MATERIALIZED VIEW CONCURRENTLY $1", view_name)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
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
		attachments,
		thread_id
	FROM messages
	WHERE id = $1`, id).Scan(&msg.Id, &msg.Author.Id, &msg.Text, &msg.CreatedAt, &msg.Attachments, &msg.ThreadId)
	if err != nil {
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

	_, err = tx.Exec(`
	UPDATE messages SET
		text = 'msg deleted',
		attachments = []
	WHERE id = $1`, id)
	if err != nil {
		return err
	}

	view_name := get_view_name(board)
	_, err = tx.Exec("REFRESH MATERIALIZED VIEW CONCURRENTLY $1", view_name)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}
