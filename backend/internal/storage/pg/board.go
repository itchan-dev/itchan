package pg

import (
	"database/sql"
	"fmt"

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
	// This view store op message and several (value in config) last messages. It refreshed every time new message posted to the board
	view_name := get_view_name(shortName)
	query := `
	CREATE MATERIALIZED VIEW $1 AS
	SELECT
		t.title as thread_title,
		t.board as board,
		trc.n as n_replies,
		m.id as msg_id,
		m.author_id as author_id,
		m.text as text,
		m.created as created,
		m.attachments as attachments,
		m.thread_id as thread_id,
		CASE WHEN m.thread_id IS NULL THEN TRUE ELSE FALSE end as op,
		m.n as reply_number,
		trc.last_reply_ts as last_reply_ts,
		row_number() over(partition by t.id order by last_reply_ts) as thread_order  --for pagination
	FROM messages as m
	LEFT JOIN threads as t
		ON COALESCE(m.thread_id, m.id) = t.id -- thread_id is null for op message
	LEFT JOIN thread_reply_counter as trc
		ON t.id = trc.id
	WHERE t.board = $2
		AND (((trc.n - m.n) <= $3 AND m.thread_id is null) -- leave only n last replies without op message
		OR m.thread_id is not null) -- and op message
	`
	_, err = tx.Exec(query, view_name, shortName, s.cfg.NLastMsg)
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
		createdAt int64
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
	view_name := get_view_name(shortName)
	rows, err := s.db.Query(`
	SELECT 
		*
	FROM $1 
	WHERE n_thread BETWEEN $2 * ($3 - 1) + 1 AND $2 * $3
	`, view_name, s.cfg.ThreadsPerPage, page)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// We need this because domain.Thread doesnt have OpMsg field. First message is op message for domain.Thread
	// At first we parse every message, then implicitly place op message first in domain.Thread
	// Maybe refactor later
	type parsedMsg struct {
		threadTitle string
		board       string
		nReplies    uint
		id          int64
		authorId    int64
		text        string
		created     int64
		attachments []domain.Attachment
		threadId    int64
		op          bool
		replyNumber int
		threadOrder int64
	}
	type parsedThread struct {
		OpMsg       *parsedMsg
		LastReplies []parsedMsg
	}
	parsedView := make(map[int64]parsedThread) // key is thread_id

	for rows.Next() {
		var msg parsedMsg
		err = rows.Scan(&msg.threadTitle, &msg.board, &msg.nReplies, &msg.id, &msg.authorId, &msg.attachments, &msg.threadId, &msg.op, &msg.replyNumber, &msg.threadOrder)
		if err != nil {
			return nil, err
		}
		thread := parsedView[msg.threadId]
		if msg.op {
			thread.OpMsg = &msg
		} else {
			thread.LastReplies = append(thread.LastReplies, msg)
		}
	}
	if rows.Err() != nil {
		return nil, err
	}
	// Convert pg data to desired format
	// There are no op message for domain.Thread. First message in messages is op message
	var threads []*domain.Thread
	for _, v := range parsedView {
		var messages []*domain.Message
		messages = append(messages, &domain.Message{Id: v.OpMsg.id, Author: domain.User{Id: v.OpMsg.authorId}, Text: v.OpMsg.text, CreatedAt: v.OpMsg.created, Attachments: v.OpMsg.attachments, ThreadId: v.OpMsg.threadId}) // op message first
		for _, m := range v.LastReplies {
			messages = append(messages, &domain.Message{Id: m.id, Author: domain.User{Id: m.authorId}, Text: m.text, CreatedAt: m.created, Attachments: m.attachments, ThreadId: m.threadId})
		}
		threads = append(threads, &domain.Thread{Title: v.OpMsg.threadTitle, Messages: messages, Board: v.OpMsg.board, NumReplies: v.OpMsg.nReplies})
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
	if _, err := tx.Exec("DELETE FROM messages USING threads WHERE COALESCE(messages.thread_id, messages.id) = threads.id AND threads.board = $1", shortName); err != nil {
		return err
	}
	view_name := get_view_name(shortName)
	if _, err := tx.Exec("DROP MATERIALIZED VIEW $1", view_name); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}
