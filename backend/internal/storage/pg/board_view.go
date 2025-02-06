package pg

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Create materialized view to store op message and several last replies
// This is neccessary for fast access to board page, otherwise it would require complex queries every GetBoard request
// This view store op message and several (value in config) last messages
var view_query string = `
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
	FROM data;
	CREATE UNIQUE INDEX ON %s (msg_id);
	`

func (s *Storage) StartPeriodicViewRefresh(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				boards, err := s.GetActiveBoards(interval)
				if err != nil {
					log.Printf("Error fetching active boards: %v", err)
					continue
				}
				var wg sync.WaitGroup
				for _, board := range boards {
					wg.Add(1)
					go func(b string) {
						defer wg.Done()
						if err := s.refreshMaterializedViewConcurrent(b, interval); err != nil {
							log.Printf("Refresh failed for %s: %v", b, err)
						}
					}(board.ShortName)
				}
				wg.Wait()
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

func (s *Storage) refreshMaterializedViewConcurrent(boardShortName string, interval time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), interval*2)
	defer cancel()

	viewName := getViewName(boardShortName)
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf("REFRESH MATERIALIZED VIEW CONCURRENTLY %s", viewName),
	)
	if err != nil {
		return fmt.Errorf("concurrent refresh failed: %w", err)
	}
	return nil
}
