package pg

import (
	"database/sql"
	"fmt"
	"time"

	"errors"

	internal_errors "github.com/itchan-dev/itchan/backend/internal/errors"
	"github.com/itchan-dev/itchan/shared/domain"
	_ "github.com/lib/pq"
)

func (s *Storage) CreateBoard(name, shortName string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // The rollback will be ignored if the tx has been committed later in the function.

	_, err = tx.Exec("INSERT INTO boards(name, short_name) VALUES($1, $2)", name, shortName)
	if err != nil { // catch unique violation error and raise "user already exists"
		return err
	}

	// Create materialized view to store op message and several last replies
	// This is neccessary for fast access to board page, otherwise it would require complex queries every GetBoard request
	// This view store op message and several (value in config) last messages. It refreshed every time new message posted to the board (maybe change to refresh every sec?)
	query := fmt.Sprintf(`
	CREATE MATERIALIZED VIEW %s AS
	WITH data AS (
		SELECT -- op messages
			t.title as thread_title,
			t.reply_count as reply_count,
			t.last_bump_ts as last_bump_ts,
			t.id as thread_id,
			m.id as msg_id,
			m.author_id as author_id,
			m.text as text,
			m.created as created,
			m.attachments as attachments,
			true as op,
			m.n as reply_number
		FROM threads as t
		JOIN messages as m
			ON t.id = m.id
		WHERE t.board = '%s'
		UNION ALL
		SELECT -- last messages
			t.title as thread_title,
			t.reply_count as reply_count,
			t.last_bump_ts as last_bump_ts,
			t.id as thread_id,
			m.id as msg_id,
			m.author_id as author_id,
			m.text as text,
			m.created as created,
			m.attachments as attachments,
			false as op,
			m.n as reply_number
		FROM threads as t
		JOIN messages as m
			ON t.id = m.thread_id -- thread_id is null for op message
		WHERE t.board = '%s'
		AND (t.reply_count - m.n) < %d
	)
	SELECT
		*
		,dense_rank() over(order by last_bump_ts desc, thread_id) as thread_order  -- for pagination
	FROM data
	ORDER BY thread_order, created 
	`, getViewName(shortName), shortName, shortName, s.cfg.Public.NLastMsg)
	_, err = tx.Exec(query)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func (s *Storage) GetBoard(shortName string, page int) (*domain.Board, error) {
	// At first, get board metadata
	type metadata struct {
		name      string
		shortName string
		createdAt time.Time
	}
	var m metadata
	err := s.db.QueryRow("SELECT name, short_name, created FROM boards WHERE short_name = $1", shortName).Scan(&m.name, &m.shortName, &m.createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &internal_errors.ErrorWithStatusCode{Message: "Board not found", StatusCode: 404}
		}
		return nil, err
	}
	// Then get and parse front page data from materialized view
	rows, err := s.db.Query(fmt.Sprintf(`
	SELECT 
		*
	FROM %s
	WHERE thread_order BETWEEN $1 * ($2 - 1) + 1 AND $1 * $2
	`, getViewName(shortName)), s.cfg.Public.ThreadsPerPage, page)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var threads []*domain.Thread
	var thread domain.Thread
	var row struct {
		threadTitle string
		nReplies    uint
		lastBumpTs  time.Time
		threadId    sql.NullInt64
		msgId       int64
		authorId    int64
		text        string
		created     time.Time
		attachments *domain.Attachments
		op          bool
		replyNumber int
		threadOrder int64
	}
	for rows.Next() {
		err = rows.Scan(&row.threadTitle, &row.nReplies, &row.lastBumpTs, &row.threadId, &row.msgId, &row.authorId, &row.text, &row.created, &row.attachments, &row.op, &row.replyNumber, &row.threadOrder)
		if err != nil {
			return nil, err
		}
		if row.op { // op message means new thread begins
			if len(thread.Messages) != 0 { // check if this is first parsed row. otherwise thread have atleast 1 message
				copyThread := thread
				threads = append(threads, &copyThread)
			}
			thread = domain.Thread{Title: row.threadTitle, Board: m.shortName, NumReplies: row.nReplies, LastBumped: row.lastBumpTs}
		}
		msg := domain.Message{Id: row.msgId, Author: domain.User{Id: row.authorId}, Text: row.text, CreatedAt: row.created, Attachments: row.attachments, ThreadId: row.threadId}
		thread.Messages = append(thread.Messages, &msg)
	}
	if len(thread.Messages) != 0 {
		copyThread := thread
		threads = append(threads, &copyThread)
	}
	if rows.Err() != nil {
		return nil, err
	}
	board := domain.Board{Name: m.name, ShortName: m.shortName, Threads: threads, CreatedAt: m.createdAt}
	return &board, nil
}

func (s *Storage) DeleteBoard(shortName string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // The rollback will be ignored if the tx has been committed later in the function.

	// Cleanup messages, threads will cascade delete
	if _, err := tx.Exec("DELETE FROM messages USING threads WHERE threads.board = $1 AND COALESCE(messages.thread_id, messages.id) = threads.id", shortName); err != nil {
		return err
	}
	result, err := tx.Exec("DELETE FROM boards WHERE short_name = $1", shortName)
	if err != nil {
		return err
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if deleted == 0 {
		return &internal_errors.ErrorWithStatusCode{Message: "Board not found", StatusCode: 404}
	}
	if _, err := tx.Exec(fmt.Sprintf("DROP MATERIALIZED VIEW %s", getViewName(shortName))); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}
