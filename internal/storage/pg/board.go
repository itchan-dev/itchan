package pg

import (
	"fmt"

	"github.com/itchan-dev/itchan/internal/domain"
	_ "github.com/lib/pq"
)

// make transactional
func (s *Storage) CreateBoard(name, shortName string) error {
	_, err := s.db.Exec("INSERT INTO boards(name, short_name) VALUES($1, $2)", name, shortName)
	if err != nil {
		return err
	}
	view_name := fmt.Sprintf("board_%s_view", shortName)
	query := `CREATE MATERIALIZED VIEW $1 AS
    SELECT
        t.title as title,
        t.id as id,
        m_op.author_id as author_id,
        m_op.text as msg_txt,
        m_op.created as created,
        m_op.attachments as attachments,
        count(m.id) as n_replies,
        max(m.created) as last_reply_ts
    FROM threads as t
    LEFT JOIN messages as m_op -- one op message for thread metadata
        ON t.id = m_op.id
    LEFT JOIN messages as m
        ON t.id = m.thread_id
    WHERE t.board = $2
    GROUP BY 
        t.title,
        t.id,
        m_op.author_id,
        m_op.text,
        m_op.created,
        m_op.attachments
    ORDER BY last_reply_ts`
	_, err = s.db.Exec(query, view_name, shortName)
	return err
}

func (s *Storage) GetBoard(shortName string, page int) (*domain.Board, error) {
	type metadata struct {
		Name      string
		ShortName string
		CreatedAt int64
	}
	var m metadata
	err := s.db.QueryRow("SELECT name, short_name, created FROM boards WHERE short_name = $1", shortName).Scan(&m.Name, &m.ShortName, &m.CreatedAt)
	if err != nil {
		return nil, err
	}

	view_name := fmt.Sprintf("board_%s_view", shortName)
	rows, err := s.db.Query("SELECT title, id, author_id, op_msg, created, op_attachments, n_replies FROM $1 LIMIT $2 OFFSET ($3 - 1) * $2", view_name, s.cfg.ThreadsPerPage, page)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var threads []*domain.Thread

	for rows.Next() {
		var t domain.Thread
		var op_msg domain.Message
		err = rows.Scan(&t.Title, &op_msg.Id, &op_msg.Author.Id, &op_msg.Text, &op_msg.CreatedAt, &op_msg.Attachments, &t.NumReplies)
		if err != nil {
			return nil, err
		}
		t.Messages = []*domain.Message{&op_msg}
		threads = append(threads, &t)
	}

	// Any errors encountered by rows.Next or rows.Scan will be returned here
	if rows.Err() != nil {
		return nil, err
	}
	board := domain.Board{Name: m.Name, ShortName: m.ShortName, Threads: threads, CreatedAt: m.CreatedAt}
	return &board, nil
}

func (s *Storage) DeleteBoard(shortName string) error {
	if _, err := s.db.Exec("DELETE FROM boards WHERE short_name = $1", shortName); err != nil {
		return err
	}
	return nil
}
