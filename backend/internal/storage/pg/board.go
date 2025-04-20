package pg

import (
	"database/sql"
	"fmt"
	"time"

	"errors"

	"github.com/itchan-dev/itchan/shared/domain"
	internal_errors "github.com/itchan-dev/itchan/shared/errors"
)

var emptyAllowedEmailsError = errors.New("allowedEmails should be either nil or not empty")

func (s *Storage) CreateBoard(name, shortName string, allowedEmails *domain.Emails) error {
	if allowedEmails != nil && len(*allowedEmails) == 0 {
		return emptyAllowedEmailsError
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // The rollback will be ignored if the tx has been committed later in the function.

	_, err = tx.Exec("INSERT INTO boards(name, short_name, allowed_emails) VALUES($1, $2, $3)", name, shortName, allowedEmails)
	if err != nil { // catch unique violation error and raise "user already exists"
		return err
	}

	query := fmt.Sprintf(view_query, getViewName(shortName), shortName, shortName, s.cfg.Public.NLastMsg, getViewName(shortName))
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
		name          string
		shortName     string
		allowedEmails *domain.Emails
		createdAt     time.Time
	}
	var m metadata
	err := s.db.QueryRow("SELECT name, short_name, allowed_emails, created FROM boards WHERE short_name = $1", shortName).Scan(&m.name, &m.shortName, &m.allowedEmails, &m.createdAt)
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
	ORDER BY thread_order, created
	`, getViewName(shortName)), s.cfg.Public.ThreadsPerPage, page)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var threads []*domain.Thread
	var thread domain.Thread
	var row struct {
		threadTitle string
		nReplies    int
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
	board := domain.Board{Name: m.name, ShortName: m.shortName, Threads: threads, AllowedEmails: m.allowedEmails, CreatedAt: m.createdAt}
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

func (s *Storage) GetBoardsAllowedEmails() ([]domain.Board, error) {
	var boards []domain.Board
	rows, err := s.db.Query(`
	SELECT
		name,
		short_name,
		allowed_emails
	FROM boards
	WHERE allowed_emails IS NOT NULL
	ORDER BY created
	`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		board := domain.Board{}
		err = rows.Scan(&board.Name, &board.ShortName, &board.AllowedEmails)
		if err != nil {
			return nil, err
		}
		boards = append(boards, board)
	}
	return boards, nil
}

// Boards that had new messages at last `interval` time
func (s *Storage) GetActiveBoards(interval time.Duration) ([]domain.Board, error) {
	var boards []domain.Board
	rows, err := s.db.Query(`
	SELECT
		t.board
	FROM messages as m
	JOIN threads as t
		ON coalesce(m.thread_id, m.id) = t.id
	GROUP BY board
	HAVING EXTRACT(EPOCH FROM ((now() at time zone 'utc') - MAX(m.modified))) < $1
	`, interval.Seconds())
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		board := domain.Board{}
		// err = rows.Scan(&board.ShortName)
		err = rows.Scan(&board.ShortName)
		if err != nil {
			return nil, err
		}
		boards = append(boards, board)
	}
	return boards, nil
}

func (s *Storage) GetBoards(user *domain.User) ([]domain.Board, error) {
	var boards []domain.Board
	var err error
	var rows *sql.Rows
	if user.Admin {
		rows, err = s.db.Query(`
	SELECT
		name,
		short_name
	FROM boards
	ORDER BY created
	`)
	} else {
		// restrict boards for non-admins
		domain, err := user.Domain()
		if err != nil {
			return nil, err
		}
		rows, err = s.db.Query(`
	SELECT
		name,
		short_name
	FROM boards
	where (allowed_emails is null) or ($1 =any(allowed_emails))
	ORDER BY created
	`, domain)
	}
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		board := domain.Board{}
		err = rows.Scan(&board.Name, &board.ShortName)
		if err != nil {
			return nil, err
		}
		boards = append(boards, board)
	}
	return boards, nil
}
