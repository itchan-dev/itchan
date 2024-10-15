package pg

import (
	"fmt"

	"github.com/itchan-dev/itchan/internal/domain"
	_ "github.com/lib/pq"
)

func get_view_name(shortName string) string {
	return fmt.Sprintf("board_%s_view", shortName)
}

// transactional
func (s *Storage) CreateBoard(name, shortName string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // The rollback will be ignored if the tx has been committed later in the function.

	_, err = tx.Exec("INSERT INTO boards(name, short_name) VALUES($1, $2)", name, shortName)
	if err != nil {
		return err
	}

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
		return err
	}
	return nil
}

func (s *Storage) GetBoard(shortName string, page int) (*domain.Board, error) {
	type metadata struct {
		name      string
		shortName string
		createdAt int64
	}
	var m metadata
	err := s.db.QueryRow("SELECT name, short_name, created FROM boards WHERE short_name = $1", shortName).Scan(&m.name, &m.shortName, &m.createdAt)
	if err != nil {
		return nil, err
	}
	// get and parse front page data from materialized view
	page = max(1, page)
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
		thrd := parsedView[msg.threadId]
		if msg.op {
			thrd.OpMsg = &msg
		} else {
			thrd.LastReplies = append(thrd.LastReplies, msg)
		}
	}
	if rows.Err() != nil {
		return nil, err
	}
	// convert pg data to desired format
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

	if _, err := tx.Exec("DELETE FROM boards WHERE short_name = $1", shortName); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM messages USING threads WHERE COALESCE(messages.thread_id, messages.id) = threads.id AND threads.board = $1", shortName); err != nil {
		return err
	}
	view_name := get_view_name(shortName)
	if _, err := tx.Exec("DROP MATERIALIZED VIEW $1", view_name); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}
